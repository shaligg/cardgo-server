import http from 'k6/http';
import ws from 'k6/ws';
import { check } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

const SCENARIO = (__ENV.SCENARIO || 'S2').toUpperCase();
const API_BASE = __ENV.API_BASE || 'http://127.0.0.1:8080';
const PASSWORD = __ENV.PASSWORD || 'demo';
const CLIENT_VER = __ENV.CLIENT_VER || '1.0.0';
const HEARTBEAT_MS = Number(__ENV.HEARTBEAT_MS || 15000);
const BIZ_INTERVAL_MS = Number(__ENV.BIZ_INTERVAL_MS || defaultBizInterval(SCENARIO));
const BIZ_STOP_BEFORE_CLOSE_MS = Number(__ENV.BIZ_STOP_BEFORE_CLOSE_MS || 15000);

const loginOk = new Rate('login_ok_rate');
const wsConnectOk = new Rate('ws_connect_ok_rate');
const authOk = new Rate('ws_auth_ok_rate');
const bizAckOk = new Rate('ws_biz_ack_ok_rate');

const wsAuthLatency = new Trend('ws_auth_latency_ms');
const wsBizRTT = new Trend('ws_biz_rtt_ms');
const wsSessionLife = new Trend('ws_session_lifetime_ms');

const wsErrorEvents = new Counter('ws_error_events');
const wsServerFullEvents = new Counter('ws_server_full_events');
const wsBizSent = new Counter('ws_biz_sent_total');
const wsBizAcked = new Counter('ws_biz_acked_total');

const scenarioCfg = buildScenarioConfig(SCENARIO);

export const options = {
  scenarios: {
    online: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: scenarioCfg.stages,
      gracefulRampDown: '30s',
    },
  },
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  thresholds: buildThresholds(SCENARIO),
};

export default function () {
  const account = `k6_${SCENARIO.toLowerCase()}_${__VU}`;
  const loginRes = login(account);
  if (!loginRes.ok) {
    loginOk.add(false);
    return;
  }
  loginOk.add(true);

  let authSucceeded = false;
  let sawServerFull = false;
  const pendingBiz = {};
  const sessionStartMs = Date.now();

  const wsRes = ws.connect(loginRes.wsAddr, { tags: { scenario: SCENARIO } }, function (socket) {
    let seq = 1;
    const authSeq = seq++;
    const authSentAt = Date.now();
    const bizStopAt = Date.now() + Math.max(scenarioCfg.socketLiveMs-BIZ_STOP_BEFORE_CLOSE_MS, 0);

    socket.on('open', function () {
      socket.send(
        JSON.stringify({
          seq: authSeq,
          type: 'auth_req',
          ts: epochSec(),
          payload: {
            ticket: loginRes.ticket,
          },
        }),
      );
    });

    socket.on('message', function (raw) {
      let msg = null;
      try {
        msg = JSON.parse(raw);
      } catch (e) {
        wsErrorEvents.add(1);
        return;
      }

      if (msg.type === 'auth_ack') {
        authSucceeded = true;
        wsAuthLatency.add(Date.now() - authSentAt);
        return;
      }

      if (msg.type === 'biz_ack') {
        const sentAt = pendingBiz[msg.seq];
        if (sentAt !== undefined) {
          wsBizAcked.add(1);
          wsBizRTT.add(Date.now() - sentAt);
          bizAckOk.add(true);
          delete pendingBiz[msg.seq];
        }
        return;
      }

      if (msg.type === 'server_full') {
        sawServerFull = true;
        wsServerFullEvents.add(1);
        socket.close();
        return;
      }

      if (msg.type === 'error') {
        wsErrorEvents.add(1);
        return;
      }
    });

    socket.on('error', function () {
      wsErrorEvents.add(1);
    });

    socket.setInterval(function () {
      socket.send(
        JSON.stringify({
          seq: seq++,
          type: 'heartbeat_req',
          ts: epochSec(),
          payload: {},
        }),
      );
    }, HEARTBEAT_MS);

    socket.setInterval(function () {
      if (!authSucceeded) {
        return;
      }
      if (Date.now() >= bizStopAt) {
        return;
      }
      const bizSeq = seq++;
      pendingBiz[bizSeq] = Date.now();
      wsBizSent.add(1);

      socket.send(
        JSON.stringify({
          seq: bizSeq,
          type: 'biz_req',
          ts: epochSec(),
          payload: {
            op_code: 1001,
          },
        }),
      );
    }, BIZ_INTERVAL_MS);

    socket.setTimeout(function () {
      socket.close();
    }, scenarioCfg.socketLiveMs);
  });

  wsConnectOk.add(wsRes && wsRes.status === 101);

  if (wsRes && wsRes.status !== 101 && containsServerFull(wsRes.body)) {
    sawServerFull = true;
    wsServerFullEvents.add(1);
  }

  for (const key in pendingBiz) {
    if (Object.prototype.hasOwnProperty.call(pendingBiz, key)) {
      bizAckOk.add(false);
    }
  }

  authOk.add(authSucceeded);
  wsSessionLife.add(Date.now() - sessionStartMs);

  if (SCENARIO !== 'S3' && sawServerFull) {
    wsErrorEvents.add(1);
  }
}

function login(account) {
  const payload = {
    account,
    password: PASSWORD,
    client_ip: `10.0.${Math.floor(__VU / 255)}.${(__VU % 255) + 1}`,
    client_ver: CLIENT_VER,
  };

  const res = http.post(`${API_BASE}/api/login`, JSON.stringify(payload), {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: {
      endpoint: 'login',
      scenario: SCENARIO,
    },
  });

  const ok = check(res, {
    'login http status is 200': (r) => r.status === 200,
  });
  if (!ok) {
    return { ok: false };
  }

  let body = null;
  try {
    body = JSON.parse(res.body);
  } catch (e) {
    return { ok: false };
  }

  if (body.code !== 0 || !body.data || !body.data.ws_addr || !body.data.enter_ticket) {
    return { ok: false };
  }

  return {
    ok: true,
    wsAddr: body.data.ws_addr,
    ticket: body.data.enter_ticket,
  };
}

function buildScenarioConfig(name) {
  const defaults = {
    S1: {
      stages: [
        { duration: __ENV.S1_RAMP_UP || '2m', target: Number(__ENV.S1_TARGET || 200) },
        { duration: __ENV.S1_HOLD || '8m', target: Number(__ENV.S1_TARGET || 200) },
        { duration: __ENV.S1_RAMP_DOWN || '1m', target: 0 },
      ],
    },
    S2: {
      stages: [
        { duration: __ENV.S2_RAMP_UP || '5m', target: Number(__ENV.S2_TARGET || 2000) },
        { duration: __ENV.S2_HOLD || '30m', target: Number(__ENV.S2_TARGET || 2000) },
        { duration: __ENV.S2_RAMP_DOWN || '2m', target: 0 },
      ],
    },
    S3: {
      stages: [
        { duration: __ENV.S3_RAMP_UP || '10m', target: Number(__ENV.S3_TARGET || 2200) },
        { duration: __ENV.S3_HOLD || '1m', target: Number(__ENV.S3_TARGET || 2200) },
        { duration: __ENV.S3_RAMP_DOWN || '1m', target: 0 },
      ],
    },
  };

  const selected = defaults[name] || defaults.S2;
  const totalMs = selected.stages.reduce((sum, stage) => sum + durationToMs(stage.duration), 0);
  const socketLiveMs = Number(__ENV.SOCKET_LIVE_MS || Math.max(totalMs + 5000, 10000));

  return {
    stages: selected.stages,
    socketLiveMs,
  };
}

function buildThresholds(name) {
  const common = {
    login_ok_rate: ['rate>=0.999'],
    ws_biz_ack_ok_rate: ['rate>=0.990'],
    ws_auth_latency_ms: ['p(95)<2000'],
    ws_biz_rtt_ms: ['p(95)<50', 'p(99)<120'],
  };

  if (name === 'S3') {
    return {
      ...common,
      ws_auth_ok_rate: ['rate>=0.900'],
      ws_connect_ok_rate: ['rate>=0.900'],
      ws_server_full_events: ['count>0'],
    };
  }

  return {
    ...common,
    ws_auth_ok_rate: ['rate>=0.999'],
    ws_connect_ok_rate: ['rate>=0.995'],
    ws_server_full_events: ['count==0'],
  };
}

function defaultBizInterval(name) {
  if (name === 'S1') {
    return 5000;
  }
  if (name === 'S2') {
    return 1000;
  }
  return 1500;
}

function durationToMs(raw) {
  const m = /^([0-9]+)(ms|s|m|h)$/.exec(String(raw).trim());
  if (!m) {
    return 0;
  }
  const value = Number(m[1]);
  const unit = m[2];
  if (unit === 'ms') {
    return value;
  }
  if (unit === 's') {
    return value * 1000;
  }
  if (unit === 'm') {
    return value * 60 * 1000;
  }
  if (unit === 'h') {
    return value * 60 * 60 * 1000;
  }
  return 0;
}

function containsServerFull(body) {
  if (!body) {
    return false;
  }
  if (typeof body === 'string') {
    return body.indexOf('SERVER_FULL') >= 0 || body.indexOf('server_full') >= 0;
  }
  return false;
}

function epochSec() {
  return Math.floor(Date.now() / 1000);
}

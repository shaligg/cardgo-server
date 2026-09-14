package loginserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/platform/account"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestAccountHTTPDatabaseUnavailable 通过真实 MySQL 驱动的连接失败验证受控错误，无需停止开发数据库。
func TestAccountHTTPDatabaseUnavailable(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{DSN: "test@tcp(127.0.0.1:0)/unavailable?timeout=50ms", SkipInitializeWithVersion: true}), &gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service, err := account.NewService(repo.NewDBAccountRepository(db), idb.NewTxManager(db), 30*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	mux := buildHTTPMux(service, nil, httpTestConfig())
	for _, path := range []string{"/api/register", "/api/login", "/api/enter", "/api/logout"} {
		body := `{"account":"test_user","password":"correct-password"}`
		tokenID := uuid.NewString()
		if path == "/api/enter" || path == "/api/logout" {
			body = `{}`
		}

		r := httptest.NewRequest("POST", path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer s."+tokenID+"."+strings.Repeat("A", 43))
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 503 || !strings.Contains(w.Body.String(), "SERVICE_UNAVAILABLE") || strings.Contains(w.Body.String(), "127.0.0.1") || strings.Contains(w.Body.String(), "enter_ticket") {
			t.Fatalf("%s failure response: %d %s", path, w.Code, w.Body.String())
		}
	}
	t.Setenv("GAME_CONFIG", "../../../configs/loginserver.local.yaml")
	t.Setenv("GAME_TICKET_SECRET", "test-secret")
	t.Setenv("ACCOUNT_DB_DSN", "test@tcp(127.0.0.1:0)/unavailable")
	if _, err := Bootstrap(context.Background()); err == nil {
		t.Fatal("bootstrap accepted unavailable account database")
	}
}

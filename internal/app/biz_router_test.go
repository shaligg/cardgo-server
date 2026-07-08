package app

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/contract/protocol"
	terrors "github.com/bigfish/go_orm_1/internal/framework/transport/errors"
)

func TestBizRouterReturnsUnsupportedOpCode(t *testing.T) {
	router := newBizRouter()
	_, err := router.Handle(context.Background(), 999999, "u1", nil)
	if err == nil {
		t.Fatal("err is nil, want unsupported op_code error")
	}
	if err.Code != terrors.CodeUnsupported {
		t.Fatalf("code = %s, want %s", err.Code, terrors.CodeUnsupported)
	}
}

func TestRegisterBizRoutesCanDisableDebugOps(t *testing.T) {
	router := newBizRouter()
	registerBizRoutes(router, &playerHandler{}, &assetHandler{}, &cardHandler{}, &levelHandler{}, &workshopHandler{}, false)

	if _, ok := router.handlers[protocol.OpPlayerGetProfile]; !ok {
		t.Fatalf("player.get_profile should always be registered")
	}
	for _, opCode := range []int32{
		protocol.OpPlayerAddGold,
		protocol.OpPlayerConsumeGold,
		protocol.OpAssetGrantItem,
		protocol.OpAssetConsumeItem,
	} {
		if _, ok := router.handlers[opCode]; ok {
			t.Fatalf("debug op_code %d should not be registered when debug ops are disabled", opCode)
		}
	}
}

func TestRegisterBizRoutesCanEnableDebugOps(t *testing.T) {
	router := newBizRouter()
	registerBizRoutes(router, &playerHandler{}, &assetHandler{}, &cardHandler{}, &levelHandler{}, &workshopHandler{}, true)

	for _, opCode := range []int32{
		protocol.OpPlayerAddGold,
		protocol.OpPlayerConsumeGold,
		protocol.OpAssetGrantItem,
		protocol.OpAssetConsumeItem,
	} {
		if _, ok := router.handlers[opCode]; !ok {
			t.Fatalf("debug op_code %d should be registered when debug ops are enabled", opCode)
		}
	}
}

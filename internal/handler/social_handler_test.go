package handler

import (
	"context"
	"testing"

	"github.com/bigfish/go_orm_1/internal/globalcore"
	idb "github.com/bigfish/go_orm_1/internal/infra/db"
	"github.com/bigfish/go_orm_1/internal/repo"
	"github.com/bigfish/go_orm_1/internal/testutil/testdb"
)

func TestFriendHandlerUsesAuthenticatedUID(t *testing.T) {
	db := testdb.OpenGame(t)
	playerRepo := repo.NewDBPlayerRepository(db)
	friendRepo := repo.NewDBFriendRepository(db)
	for _, uid := range []string{"auth_uid", "target_uid", "evil_uid"} {
		if _, err := playerRepo.CreateIfAbsent(context.Background(), repo.Player{
			UID: uid, Nickname: "Nick_" + uid, AvatarID: 1, Level: 1,
		}); err != nil {
			t.Fatalf("create player %s: %v", uid, err)
		}
	}
	friendService := globalcore.LocalFriendService{Repo: friendRepo, Tx: idb.NewTxManager(db)}
	h := &BizHandler{FriendService: friendService}

	_, bizErr := h.FriendApply(context.Background(), "auth_uid", []byte(`{"uid":"evil_uid","target_uid":"target_uid","req_id":"friend-handler"}`))
	if bizErr != nil {
		t.Fatalf("FriendApply: %v", bizErr)
	}
	authItems, _, err := friendService.List(context.Background(), "auth_uid", "", 20)
	if err != nil || len(authItems) != 1 || authItems[0].UID != "target_uid" {
		t.Fatalf("auth friend list = %#v err=%v", authItems, err)
	}
	evilItems, _, err := friendService.List(context.Background(), "evil_uid", "", 20)
	if err != nil || len(evilItems) != 0 {
		t.Fatalf("evil friend list = %#v err=%v", evilItems, err)
	}
}

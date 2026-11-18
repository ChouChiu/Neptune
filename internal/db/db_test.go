package db

import (
	"errors"
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	t.Chdir("../..")
	database, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() {
		_ = database.Close()
	})
	if err := database.InitDatabase(); err != nil {
		t.Fatalf("InitDatabase() error = %v", err)
	}
	return database
}

func TestNewAndInitDatabase(t *testing.T) {
	database := openTestDB(t)

	var journalMode string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil || journalMode != "wal" {
		t.Fatalf("journal_mode = %q, err = %v", journalMode, err)
	}
	var fk int
	if err := database.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil || fk != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", fk, err)
	}
	if err := database.InitDatabase(); err != nil {
		t.Fatalf("second InitDatabase() error = %v", err)
	}
}

func TestAdminAuthorizationIsolation(t *testing.T) {
	database := openTestDB(t)
	const (
		adminID      int64 = 1001
		boundGroup   int64 = -100100
		unboundGroup int64 = -100200
	)

	if err := database.InitGroup(unboundGroup); err != nil {
		t.Fatalf("InitGroup() error = %v", err)
	}
	if err := database.ConnectAdmin(adminID, boundGroup); err != nil {
		t.Fatalf("ConnectAdmin() error = %v", err)
	}
	if err := database.SetCurrentGroup(adminID, unboundGroup); !errors.Is(err, ErrAdminNotConnected) {
		t.Fatalf("SetCurrentGroup(unbound) error = %v, want ErrAdminNotConnected", err)
	}

	// Even if admin_current_group contains an unbound row, GetAdminGroupID must reject it without fallback.
	if _, err := database.Exec("INSERT OR REPLACE INTO admin_current_group (user_id, group_id) VALUES (?, ?)", adminID, unboundGroup); err != nil {
		t.Fatalf("seed dirty admin_current_group: %v", err)
	}
	got, err := database.GetAdminGroupID(adminID)
	if err != nil || got != nil {
		t.Fatalf("GetAdminGroupID() = (%v, %v), want (nil, nil)", got, err)
	}
}

func TestVerificationLifecycleAndAttempts(t *testing.T) {
	database := openTestDB(t)
	const (
		userID         int64 = 5001
		unspokenUserID int64 = 5002
		groupID        int64 = -100500
	)
	welcomeMsgID := int64(42)
	expiresAt := currentTimestamp() + 300

	if err := database.AddPendingVerification(userID, groupID, "", expiresAt, &welcomeMsgID, false); err != nil {
		t.Fatalf("AddPendingVerification() error = %v", err)
	}
	// Non-captcha records must not match DM captcha replies or consume attempts.
	if active, err := database.GetPendingVerificationsByUser(userID); err != nil || len(active) != 0 {
		t.Fatalf("GetPendingVerificationsByUser() = (%d, %v), want 0", len(active), err)
	}
	if err := database.IncrementVerificationAttempts(userID); err != nil {
		t.Fatalf("IncrementVerificationAttempts() error = %v", err)
	}

	if err := database.SetPendingCaptcha(userID, groupID, "ABCDE", expiresAt, nil, true); err != nil {
		t.Fatalf("SetPendingCaptcha() error = %v", err)
	}
	if err := database.IncrementVerificationAttempts(userID); err != nil {
		t.Fatalf("IncrementVerificationAttempts() error = %v", err)
	}
	// Refreshing captcha must preserve attempts and welcome_message_id.
	if err := database.SetPendingCaptcha(userID, groupID, "FGHIJ", expiresAt+60, nil, true); err != nil {
		t.Fatalf("second SetPendingCaptcha() error = %v", err)
	}
	pv, err := database.GetPendingVerification(userID, groupID)
	if err != nil || pv == nil || pv.Attempts != 1 || pv.RuleAckDone != 1 || pv.CaptchaText != "FGHIJ" ||
		pv.WelcomeMessageID == nil || *pv.WelcomeMessageID != welcomeMsgID {
		t.Fatalf("unexpected pending verification state: %+v, err=%v", pv, err)
	}

	// Re-joining resets attempts and prompted_on_message.
	if err := database.AddPendingVerification(userID, groupID, "", expiresAt+120, &welcomeMsgID, false); err != nil {
		t.Fatalf("rejoin AddPendingVerification() error = %v", err)
	}

	// First group message transitions PromptedOnMessage 0 -> 1 once and updates welcome_message_id.
	if first, err := database.MarkFirstMessagePrompted(userID, groupID, expiresAt+240); err != nil || !first {
		t.Fatalf("first MarkFirstMessagePrompted() = (%v, %v), want (true, nil)", first, err)
	}
	if second, err := database.MarkFirstMessagePrompted(userID, groupID, expiresAt+300); err != nil || second {
		t.Fatalf("second MarkFirstMessagePrompted() = (%v, %v), want (false, nil)", second, err)
	}
	secondMsgID := int64(84)
	if err := database.UpdatePendingWelcomeMessageID(userID, groupID, &secondMsgID); err != nil {
		t.Fatalf("UpdatePendingWelcomeMessageID() error = %v", err)
	}
	pv, err = database.GetPendingVerification(userID, groupID)
	if err != nil || pv == nil || pv.PromptedOnMessage != 1 || pv.Attempts != 0 ||
		pv.WelcomeMessageID == nil || *pv.WelcomeMessageID != secondMsgID {
		t.Fatalf("unexpected state after first message: %+v, err=%v", pv, err)
	}

	// Unprompted expired join records survive cleanup; prompted expired records are removed.
	if err := database.AddPendingVerification(unspokenUserID, groupID, "", currentTimestamp()-10, &welcomeMsgID, false); err != nil {
		t.Fatalf("AddPendingVerification(unspoken) error = %v", err)
	}
	if err := database.CleanExpiredVerifications(); err != nil {
		t.Fatalf("CleanExpiredVerifications() error = %v", err)
	}
	if unspoken, err := database.GetPendingVerification(unspokenUserID, groupID); err != nil || unspoken == nil {
		t.Fatalf("unspoken verification should remain, got (%v, %v)", unspoken, err)
	}

	if _, err := database.MarkFirstMessagePrompted(unspokenUserID, groupID, currentTimestamp()-10); err != nil {
		t.Fatalf("MarkFirstMessagePrompted(unspoken) error = %v", err)
	}
	if err := database.CleanExpiredVerifications(); err != nil {
		t.Fatalf("CleanExpiredVerifications() after prompt error = %v", err)
	}
	if unspoken, err := database.GetPendingVerification(unspokenUserID, groupID); err != nil || unspoken != nil {
		t.Fatalf("expected expired prompted verification to be cleaned, got (%v, %v)", unspoken, err)
	}
}

func TestVotekickReportAndLock(t *testing.T) {
	database := openTestDB(t)
	const (
		groupID int64 = -100777
		voteID        = "vote_abc123"
	)

	now := currentTimestamp()
	if err := database.CreateActiveVote(voteID, groupID, 2001, 2002, now, now+300); err != nil {
		t.Fatalf("CreateActiveVote() error = %v", err)
	}
	if added, err := database.AddVoteRecord(voteID, 3001, 1); err != nil || !added {
		t.Fatalf("first AddVoteRecord() = (%v, %v), want (true, nil)", added, err)
	}
	if dup, err := database.AddVoteRecord(voteID, 3001, 0); err != nil || dup {
		t.Fatalf("duplicate AddVoteRecord() = (%v, %v), want (false, nil)", dup, err)
	}
	if _, err := database.AddVoteRecord(voteID, 3002, 0); err != nil {
		t.Fatalf("second AddVoteRecord() error = %v", err)
	}
	if yes, no, err := database.GetVoteCounts(voteID); err != nil || yes != 1 || no != 1 {
		t.Fatalf("GetVoteCounts() = (%d, %d, %v), want (1, 1, nil)", yes, no, err)
	}
	if err := database.DeleteActiveVote(voteID); err != nil {
		t.Fatalf("DeleteActiveVote() error = %v", err)
	}

	if err := database.AddReport(groupID, 2002, 2001, nil, "text", "spam"); err != nil {
		t.Fatalf("AddReport() error = %v", err)
	}
	reports, err := database.GetReports(nil, nil)
	if err != nil || len(reports) != 1 {
		t.Fatalf("GetReports() = %d, err=%v", len(reports), err)
	}
	if ok1, err := database.ResolvePendingReport(reports[0].ID, "approved", 1001); err != nil || !ok1 {
		t.Fatalf("first ResolvePendingReport() = (%v, %v), want (true, nil)", ok1, err)
	}
	if ok2, err := database.ResolvePendingReport(reports[0].ID, "dismissed", 1001); err != nil || ok2 {
		t.Fatalf("second ResolvePendingReport() = (%v, %v), want (false, nil)", ok2, err)
	}

	if ok, err := database.AcquireLock("welcome:-100777:2001", 10); err != nil || !ok {
		t.Fatalf("first AcquireLock() = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := database.AcquireLock("welcome:-100777:2001", 10); err != nil || ok {
		t.Fatalf("second AcquireLock() = (%v, %v), want (false, nil)", ok, err)
	}
	if err := database.ReleaseLock("welcome:-100777:2001"); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}
}

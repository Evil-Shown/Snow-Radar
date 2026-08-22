package billing

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
	"time"
)

func paddleSign(secret string, ts int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + ":"))
	mac.Write(body)
	return "ts=" + strconv.FormatInt(ts, 10) + ";h1=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyPaddleAcceptsValidSignature(t *testing.T) {
	secret := "pdl_ntfset_test_secret"
	body := []byte(`{"event_type":"subscription.updated","data":{"id":"sub_1","status":"active","custom_data":{"user_id":"u-9"}}}`)
	header := paddleSign(secret, time.Now().Unix(), body)

	if err := VerifyPaddle(secret, header, body); err != nil {
		t.Fatalf("valid signature rejected: %v", err)
	}
}

func TestVerifyPaddleRejectsTamperedBody(t *testing.T) {
	secret := "s3cret"
	body := []byte(`{"amount": 100}`)
	header := paddleSign(secret, time.Now().Unix(), body)
	if err := VerifyPaddle(secret, header, []byte(`{"amount": 1}`)); err == nil {
		t.Fatal("tampered payload accepted")
	}
}

func TestVerifyPaddleRejectsReplay(t *testing.T) {
	secret := "s3cret"
	old := time.Now().Add(-10 * time.Minute).Unix()
	body := []byte(`{}`)
	header := paddleSign(secret, old, body)
	if err := VerifyPaddle(secret, header, body); err == nil {
		t.Fatal("stale timestamp accepted (replay)")
	}
}

func TestParsePaddleNormalizesEvent(t *testing.T) {
	raw := []byte(`{"event_type":"subscription.updated","data":{"id":"sub_42","status":"canceled","custom_data":{"user_id":"u-7"}}}`)
	ev, err := ParsePaddle(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Provider != "paddle" || ev.External != "sub_42" || ev.UserID != "u-7" || ev.State != StateCancelled {
		t.Fatalf("normalized event wrong: %+v", ev)
	}
}

func TestParsePaddleRejectsMissingUserID(t *testing.T) {
	raw := []byte(`{"event_type":"x","data":{"id":"sub_1","status":"active"}}`)
	if _, err := ParsePaddle(raw); err == nil {
		t.Fatal("payload without custom_data/user_id must be rejected")
	}
}

func payHereExpectedHash(secret, merchantID, orderID, amount, currency, statusCode string) string {
	md5upper := func(s string) string {
		sum := md5.Sum([]byte(s))
		return strings.ToUpper(hex.EncodeToString(sum[:]))
	}
	return md5upper(merchantID + orderID + amount + currency + md5upper(secret) + statusCode)
}

func TestPayHereHashVerification(t *testing.T) {
	secret := "merchantsecret"
	want := payHereExpectedHash(secret, "MID", "ORDER1", "1000.00", "LKR", "2")

	if !VerifyPayHere(secret, "MID", "ORDER1", "1000.00", "LKR", "2", want) {
		t.Fatal("valid PayHere hash rejected")
	}
	if VerifyPayHere(secret, "MID", "ORDER1", "9999.00", "LKR", "2", want) {
		t.Fatal("hash for different amount accepted")
	}
	if VerifyPayHere(secret, "MID", "ORDER1", "1000.00", "LKR", "-1", want) {
		t.Fatal("hash for different status accepted")
	}
}

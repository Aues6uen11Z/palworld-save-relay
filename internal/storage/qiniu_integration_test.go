package storage

import (
	"bytes"
	"context"
	"os"
	"testing"
)

// TestQiniuIntegration round-trips a small object against a real Qiniu bucket.
// Skipped unless PALRELAY_QINIU_AK/SK/BUCKET/REGION[/DOMAIN] are set.
func TestQiniuIntegration(t *testing.T) {
	ak := os.Getenv("PALRELAY_QINIU_AK")
	sk := os.Getenv("PALRELAY_QINIU_SK")
	bucket := os.Getenv("PALRELAY_QINIU_BUCKET")
	if ak == "" || sk == "" || bucket == "" {
		t.Skip("set PALRELAY_QINIU_AK/SK/BUCKET/REGION[/DOMAIN] to run")
	}
	q, err := NewQiniu(QiniuConfig{
		AccessKey: ak, SecretKey: sk, Bucket: bucket,
		Region: os.Getenv("PALRELAY_QINIU_REGION"), Domain: os.Getenv("PALRELAY_QINIU_DOMAIN"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "saves/_relay_test/" + t.Name() + ".bin"
	payload := []byte("pal-relay integration test \xe4\xb8\xad\xe6\x96\x87")

	if err := q.Put(ctx, key, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}
	t.Cleanup(func() { q.Delete(ctx, key) })

	got, err := q.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get mismatch: got %q want %q", got, payload)
	}

	objs, err := q.List(ctx, "saves/_relay_test/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	found := false
	for _, o := range objs {
		if o.Key == key {
			found = true
		}
	}
	if !found {
		t.Errorf("List did not include the uploaded key")
	}
}

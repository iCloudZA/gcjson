package gcjson_test

import (
	"context"
	"github.com/iCloudZA/gcjson"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestExample_FromHTTP(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/json", nil)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body) // 包外拿到 []byte
	if err != nil {
		t.Fatal(err)
	}

	// 直接喂给 gcjson
	title := gcjson.AnyOrAsFast[string](b, "slideshow.title", "")
	if title == "" {
		t.Fatalf("empty title")
	}
}

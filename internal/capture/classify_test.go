package capture

import (
	"testing"

	"ooblivion/internal/models"
)

func TestBuildAlertMarkdownV2(t *testing.T) {
	req := models.Request{
		ID:        123,
		Method:    "GET",
		Host:      "acme.corp",
		Path:      "/data",
		Query:     "saved=1",
		CreatedAt: "2026-08-23T04:00:33Z",
	}
	got := buildAlert(req, "saved-query", "https://ooblivion.acme.corp")
	want := "*OOBlivion Alert*\n\nAlert Name: saved\\-query\nTime: 2026\\-08\\-23T04:00:33Z\n\n`GET acme.corp/data?saved=1`\n\nView Url: [https://ooblivion\\.acme\\.corp/admin/requests/123](https://ooblivion.acme.corp/admin/requests/123)"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

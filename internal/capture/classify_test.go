package capture

import (
	"fmt"
	"testing"

	"ooblivion/internal/models"
	"ooblivion/internal/telegram"
)

func TestBuildAlertMarkdownV2(t *testing.T) {
	req := models.Request{
		ID:        123,
		Method:    "GET",
		Host:      "acme.corp",
		Path:      "/data",
		Query:     "saved=1",
		SourceIP:  "198.51.100.77",
		CreatedAt: "2026-08-23T04:00:33Z",
	}
	got := buildAlert(req, "saved-query", "https://ooblivion.acme.corp")
	want := fmt.Sprintf(
		"*OOBlivion Alert*\n\nAlert Name: saved\\-query\nTime: %s\n\nIP: `198.51.100.77`\n`GET acme.corp/data?saved=1`\n\nView Url: [https://ooblivion\\.acme\\.corp/admin/requests/123](https://ooblivion.acme.corp/admin/requests/123)",
		telegram.EscapeMD(formatTimeID(req.CreatedAt)),
	)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

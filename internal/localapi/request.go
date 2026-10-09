package localapi

import "yandu/internal/cloudsync"

func structRequest(site string) cloudsync.Request {
	return cloudsync.Request{Action: "status", SiteID: site}
}

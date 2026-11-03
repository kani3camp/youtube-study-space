package main

func testGet(key string) string {
	switch key {
	case "MYPAGE_ENVIRONMENT":
		return "development"
	case "GOOGLE_CLOUD_PROJECT":
		return "demo-youtube-study-space-ci"
	}
	return ""
}

package web

// landingSamplePosts are fixed examples on the public landing page.
func landingSamplePosts() []PostView {
	return []PostView{{
		Username:    "لیلی",
		WriterLabel: "لیلی",
		Title:       "آخی تا قیومت!",
		Lang:        "fa",
		Dir:         "rtl",
		URL:         "https://leily.waldi.blog/mother-language?src=feed",
	}, {
		Username:    "Amin",
		WriterLabel: "Amin",
		Title:       "Serendipity",
		Lang:        "en",
		Dir:         "ltr",
		URL:         "https://amin.waldi.blog/serendipity?src=feed",
	}}
}

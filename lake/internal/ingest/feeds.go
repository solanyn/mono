package ingest

import "time"

type FeedCategory string

const (
	CatTechAI     FeedCategory = "tech-ai"
	CatEngBlog    FeedCategory = "eng-blog"
	CatAggregator FeedCategory = "aggregator"
	CatInfra      FeedCategory = "infra"
	CatYouTube    FeedCategory = "youtube"
	CatNews       FeedCategory = "news"
	CatFinance    FeedCategory = "finance"
)

type Feed struct {
	Name     string
	Slug     string
	URL      string
	Category FeedCategory
	Interval time.Duration
}

var Feeds = []Feed{
	// Tech/AI (6h)
	{Name: "Simon Willison", Slug: "simon-willison", URL: "https://simonwillison.net/atom/everything/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Chip Huyen", Slug: "chip-huyen", URL: "https://huyenchip.com/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Pragmatic Engineer", Slug: "pragmatic-engineer", URL: "https://newsletter.pragmaticengineer.com/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Eugene Yan", Slug: "eugene-yan", URL: "https://eugeneyan.com/rss/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Lilian Weng", Slug: "lilian-weng", URL: "https://lilianweng.github.io/lil-log/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "DeepMind", Slug: "deepmind", URL: "https://deepmind.com/blog/feed/basic/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "NVIDIA Dev", Slug: "nvidia-dev", URL: "https://developer.nvidia.com/blog/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "CMU ML", Slug: "cmu-ml", URL: "https://blog.ml.cmu.edu/feed/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "geohot", Slug: "geohot", URL: "https://geohot.github.io/blog/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Architecture Notes", Slug: "architecture-notes", URL: "https://architecturenotes.co/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Martin Kleppmann", Slug: "martin-kleppmann", URL: "https://feeds.feedburner.com/martinkl?format=xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Josh Comeau", Slug: "josh-comeau", URL: "https://joshwcomeau.com/rss.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Latent.Space", Slug: "latent-space", URL: "https://www.latent.space/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Interconnects AI", Slug: "interconnects-ai", URL: "https://www.interconnects.ai/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "The Gradient", Slug: "the-gradient", URL: "https://thegradient.pub/rss/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "ByteByteGo", Slug: "bytebytego", URL: "https://blog.bytebytego.com/feed", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "MIT AI", Slug: "mit-ai", URL: "http://news.mit.edu/rss/topic/artificial-intelligence2", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "OpenAI", Slug: "openai", URL: "https://openai.com/blog/rss.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Anthropic", Slug: "anthropic", URL: "https://www.anthropic.com/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "Google AI", Slug: "google-ai", URL: "https://blog.google/technology/ai/rss/", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "HuggingFace", Slug: "huggingface", URL: "https://huggingface.co/blog/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},
	{Name: "PyTorch", Slug: "pytorch", URL: "https://pytorch.org/blog/feed.xml", Category: CatTechAI, Interval: 6 * time.Hour},

	// Eng Blogs (6h)
	{Name: "Netflix", Slug: "netflix", URL: "http://techblog.netflix.com/feeds/posts/default", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Spotify", Slug: "spotify", URL: "https://engineering.atspotify.com/feed/", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Meta", Slug: "meta", URL: "https://engineering.fb.com/feed/", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Airbnb", Slug: "airbnb", URL: "https://medium.com/feed/airbnb-engineering", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Pinterest", Slug: "pinterest", URL: "https://medium.com/feed/pinterest-engineering", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Dropbox", Slug: "dropbox", URL: "https://dropbox.tech/feed", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Canva", Slug: "canva", URL: "https://www.canva.dev/blog/engineering/feed.xml", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Stripe", Slug: "stripe", URL: "https://stripe.com/blog/feed.rss", Category: CatEngBlog, Interval: 6 * time.Hour},
	{Name: "Cloudflare", Slug: "cloudflare", URL: "https://blog.cloudflare.com/rss/", Category: CatEngBlog, Interval: 6 * time.Hour},

	// Aggregators
	{Name: "Hacker News", Slug: "hacker-news", URL: "https://news.ycombinator.com/rss", Category: CatAggregator, Interval: 1 * time.Hour},
	{Name: "Lobsters", Slug: "lobsters", URL: "https://lobste.rs/rss", Category: CatAggregator, Interval: 1 * time.Hour},
	{Name: "arXiv cs.CL", Slug: "arxiv-cs-cl", URL: "https://rss.arxiv.org/rss/cs.CL", Category: CatAggregator, Interval: 24 * time.Hour},
	{Name: "arXiv cs.LG", Slug: "arxiv-cs-lg", URL: "https://rss.arxiv.org/rss/cs.LG", Category: CatAggregator, Interval: 24 * time.Hour},

	// Infra/K8s (6h)
	{Name: "LWKD", Slug: "lwkd", URL: "https://lwkd.info/feed.xml", Category: CatInfra, Interval: 6 * time.Hour},
	{Name: "ghuntley", Slug: "ghuntley", URL: "https://ghuntley.com/rss/", Category: CatInfra, Interval: 6 * time.Hour},
	{Name: "Kubernetes", Slug: "kubernetes", URL: "https://kubernetes.io/feed.xml", Category: CatInfra, Interval: 6 * time.Hour},
	{Name: "CNCF Blog", Slug: "cncf-blog", URL: "https://www.cncf.io/blog/feed/", Category: CatInfra, Interval: 6 * time.Hour},
	{Name: "Cilium", Slug: "cilium", URL: "https://cilium.io/blog/rss.xml", Category: CatInfra, Interval: 6 * time.Hour},
	{Name: "Rook", Slug: "rook", URL: "https://rook.io/blog/feed.xml", Category: CatInfra, Interval: 6 * time.Hour},

	// YouTube (24h)
	{Name: "CNCF", Slug: "cncf-youtube", URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UCvqbFHwN-nwalWPjPUKpvTA", Category: CatYouTube, Interval: 24 * time.Hour},
	{Name: "Kubeflow", Slug: "kubeflow-youtube", URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UCReYvyLo2xacoE5lIqsw3fw", Category: CatYouTube, Interval: 24 * time.Hour},
	{Name: "PyData", Slug: "pydata-youtube", URL: "https://www.youtube.com/feeds/videos.xml?channel_id=UCOjD18EJYcsBog4IozkF_7w", Category: CatYouTube, Interval: 24 * time.Hour},

	// Australian News (1h)
	{Name: "ABC Just In", Slug: "abc-just-in", URL: "https://www.abc.net.au/news/feed/51120/rss.xml", Category: CatNews, Interval: 1 * time.Hour},
	{Name: "Guardian AU", Slug: "guardian-au", URL: "https://www.theguardian.com/australia-news/rss", Category: CatNews, Interval: 1 * time.Hour},
	{Name: "OzBargain", Slug: "ozbargain", URL: "https://www.ozbargain.com.au/deals/feed", Category: CatNews, Interval: 1 * time.Hour},

	// Finance (6h)
	{Name: "RBA Releases", Slug: "rba-releases", URL: "https://www.rba.gov.au/rss/rss-cb-media-releases.xml", Category: CatFinance, Interval: 6 * time.Hour},
}

package survey

import "strings"

// What the rules know about packages, services and hosts. Package names are npm names or
// normalised Python names (lower case, - for _ and .).

var frameworkDeps = map[string]string{
	"vite": "vite", "react": "react", "next": "nextjs", "@tanstack/react-start": "tanstack-start",
	"@tanstack/react-router": "tanstack-router", "react-router-dom": "react-router",
	"react-router": "react-router", "react-scripts": "create-react-app", "express": "express",
	"fastify": "fastify", "hono": "hono", "koa": "koa", "@nestjs/core": "nestjs", "vue": "vue",
	"svelte": "svelte", "@sveltejs/kit": "sveltekit", "astro": "astro", "@remix-run/react": "remix",
	"expo": "expo", "@angular/core": "angular", "flask": "flask", "fastapi": "fastapi",
	"streamlit": "streamlit", "django": "django", "gradio": "gradio", "dash": "dash",
}

// serverFrameworks are the frameworks that run a server of their own, most specific first.
var serverFrameworks = []string{
	"nextjs", "tanstack-start", "remix", "sveltekit", "nestjs", "express", "fastify", "hono",
	"koa", "django", "flask", "fastapi", "streamlit", "gradio", "dash", "go",
}

var (
	// pageFrameworks draw their pages on the server in Python.
	pageFrameworks = []string{"django", "streamlit", "gradio", "dash"}
	// fullStackFrameworks are one framework for the pages and the server.
	fullStackFrameworks = []string{"nextjs", "tanstack-start", "remix", "sveltekit"}
	// spaFrameworks build a front end whose code all runs in the browser.
	spaFrameworks   = []string{"vite", "create-react-app"}
	frontFrameworks = []string{"react", "vue", "svelte", "angular"}
)

var storeDeps = map[string]string{
	"@supabase/supabase-js": "supabase", "@supabase/ssr": "supabase", "supabase": "supabase",
	"@supabase/auth-helpers-nextjs": "supabase", "@supabase/auth-helpers-react": "supabase",
	"pg": "postgres", "postgres": "postgres", "pg-promise": "postgres", "@neondatabase/serverless": "postgres",
	"@vercel/postgres": "postgres", "psycopg2": "postgres", "psycopg2-binary": "postgres",
	"psycopg": "postgres", "asyncpg": "postgres",
	"mysql": "mysql", "mysql2": "mysql", "pymysql": "mysql", "mysqlclient": "mysql", "mysqldb": "mysql",
	"mysql-connector-python": "mysql", "mssql": "mssql", "tedious": "mssql", "pyodbc": "mssql", "pymssql": "mssql",
	"mongoose": "mongodb", "mongodb": "mongodb", "pymongo": "mongodb", "motor": "mongodb",
	"firebase": "firebase", "firebase-admin": "firebase",
	"@upstash/redis": "redis", "@vercel/kv": "redis", "redis": "redis", "ioredis": "redis",
	"better-sqlite3": "sqlite", "sqlite3": "sqlite", "sqlite": "sqlite",
	"llocal.app/kit": "sqlite", "modernc.org/sqlite": "sqlite", "github.com/mattn/go-sqlite3": "sqlite",
	"github.com/jackc/pgx/v5": "postgres", "github.com/lib/pq": "postgres", "github.com/go-sql-driver/mysql": "mysql",
}

var schemeKinds = map[string]string{
	"postgres": "postgres", "postgresql": "postgres", "mysql": "mysql", "mariadb": "mysql",
	"mssql": "mssql", "sqlserver": "mssql", "mongodb": "mongodb", "redis": "redis", "rediss": "redis",
}

// sqlKinds are the stores that tables written in SQL or an ORM belong to, by preference.
var sqlKinds = []string{"postgres", "mysql", "mssql", "sqlite", "supabase"}

type service struct {
	name, label, kind string
	packages          []string
	hosts             []string
	// env is the part of a variable's name that ties it to the service, like STRIPE.
	env string
	// key is the variable llocal asks for when the code reads none of its own.
	key string
	// fallback is only the service when no other of its kind is found: the AI SDK's own
	// package comes with every provider's.
	fallback bool
	// calls is how the service reaches the app, if it does: webhook (it sends events),
	// redirect (people sign in to it and come back) or origin (it checks which websites
	// use its key). tell is where to paste the app's new address for it.
	calls, tell string
}

var services = []service{
	{name: "stripe", label: "Stripe", kind: "payments", packages: []string{"stripe", "@stripe/stripe-js", "@stripe/react-stripe-js"},
		hosts: []string{"js.stripe.com", "api.stripe.com"}, env: "STRIPE",
		calls: "webhook", tell: "Stripe dashboard > Developers > Webhooks: change the endpoint's address"},
	{name: "paypal", label: "PayPal", kind: "payments", packages: []string{"@paypal/react-paypal-js", "@paypal/paypal-js", "@paypal/checkout-server-sdk", "paypalrestsdk"},
		hosts: []string{"paypal.com/sdk"}, env: "PAYPAL", calls: "webhook", tell: "PayPal developer dashboard > Apps & Credentials > your app > Webhooks"},
	{name: "square", label: "Square", kind: "payments", packages: []string{"square", "squareup"}, hosts: []string{"connect.squareup.com"}, env: "SQUARE",
		calls: "webhook", tell: "Square developer dashboard > your app > Webhooks"},
	{name: "openai", label: "OpenAI", kind: "AI", packages: []string{"openai", "@ai-sdk/openai"}, hosts: []string{"api.openai.com"}, env: "OPENAI"},
	{name: "anthropic", label: "Anthropic (Claude)", kind: "AI", packages: []string{"@anthropic-ai/sdk", "anthropic", "@ai-sdk/anthropic"}, hosts: []string{"api.anthropic.com"}, env: "ANTHROPIC"},
	{name: "gemini", label: "Google Gemini", kind: "AI", packages: []string{"@google/generative-ai", "@google/genai", "google-generativeai", "google-genai", "@ai-sdk/google"},
		hosts: []string{"generativelanguage.googleapis.com"}, env: "GEMINI"},
	{name: "lovable-ai", label: "Lovable AI", kind: "AI", hosts: []string{"ai.gateway.lovable.dev"}, env: "LOVABLE_API"},
	{name: "ai-gateway", label: "Vercel AI Gateway", kind: "AI", packages: []string{"ai", "@ai-sdk/gateway"}, env: "AI_GATEWAY", key: "AI_GATEWAY_API_KEY", fallback: true},
	{name: "quickbooks", label: "QuickBooks", kind: "accounting", packages: []string{"node-quickbooks", "intuit-oauth", "python-quickbooks"},
		hosts: []string{"quickbooks.api.intuit.com", "appcenter.intuit.com"}, env: "QUICKBOOKS",
		calls: "redirect", tell: "Intuit developer portal > your app > Keys and credentials > Redirect URIs: add the new address"},
	{name: "xero", label: "Xero", kind: "accounting", packages: []string{"xero-node", "xero-python"}, hosts: []string{"api.xero.com", "login.xero.com"}, env: "XERO",
		calls: "redirect", tell: "Xero developer portal > My Apps > your app > Redirect URIs: add the new address"},
	{name: "twilio", label: "Twilio", kind: "text messages", packages: []string{"twilio"}, hosts: []string{"api.twilio.com"}, env: "TWILIO",
		calls: "webhook", tell: "Twilio console > Phone Numbers > your number > Messaging, \"A message comes in\""},
	{name: "resend", label: "Resend", kind: "email", packages: []string{"resend"}, hosts: []string{"api.resend.com"}, env: "RESEND"},
	{name: "sendgrid", label: "SendGrid", kind: "email", packages: []string{"@sendgrid/mail", "sendgrid"}, hosts: []string{"api.sendgrid.com"}, env: "SENDGRID"},
	{name: "postmark", label: "Postmark", kind: "email", packages: []string{"postmark"}, hosts: []string{"api.postmarkapp.com"}, env: "POSTMARK"},
	{name: "mailgun", label: "Mailgun", kind: "email", packages: []string{"mailgun.js", "mailgun-js"}, hosts: []string{"api.mailgun.net"}, env: "MAILGUN"},
	{name: "smtp", label: "an email server (SMTP)", kind: "email", packages: []string{"nodemailer", "flask-mail", "fastapi-mail"}, env: "SMTP"},
	{name: "google-maps", label: "Google Maps", kind: "maps", packages: []string{"@react-google-maps/api", "@googlemaps/js-api-loader", "@vis.gl/react-google-maps", "googlemaps"},
		hosts: []string{"maps.googleapis.com"}, env: "GOOGLE_MAPS",
		calls: "origin", tell: "Google Cloud console > APIs & Services > Credentials > the key > Website restrictions: add the new address"},
	{name: "mapbox", label: "Mapbox", kind: "maps", packages: []string{"mapbox-gl", "react-map-gl", "@mapbox/mapbox-sdk"}, hosts: []string{"api.mapbox.com"}, env: "MAPBOX",
		calls: "origin", tell: "Mapbox account > Tokens > the token > URL restrictions: add the new address"},
	{name: "google-sheets", label: "Google Sheets", kind: "spreadsheets", packages: []string{"googleapis", "google-spreadsheet", "gspread"}, hosts: []string{"sheets.googleapis.com"}, env: "GOOGLE_SHEETS"},
	{name: "airtable", label: "Airtable", kind: "spreadsheets", packages: []string{"airtable", "pyairtable"}, hosts: []string{"api.airtable.com"}, env: "AIRTABLE"},
	{name: "slack", label: "Slack", kind: "team chat", packages: []string{"@slack/web-api", "@slack/bolt", "slack-sdk", "slack-bolt"}, hosts: []string{"hooks.slack.com", "slack.com/api"}, env: "SLACK",
		calls: "webhook", tell: "api.slack.com/apps > your app > Event Subscriptions > Request URL"},
	{name: "hubspot", label: "HubSpot", kind: "customer records", packages: []string{"@hubspot/api-client", "hubspot-api-client"}, hosts: []string{"api.hubapi.com"}, env: "HUBSPOT",
		calls: "webhook", tell: "HubSpot > Settings > Integrations > Private Apps > your app > Webhooks"},
	{name: "shopify", label: "Shopify", kind: "online store", packages: []string{"@shopify/shopify-api", "shopify-api-node", "shopifyapi"}, hosts: []string{"myshopify.com"}, env: "SHOPIFY",
		calls: "webhook", tell: "Shopify admin > Settings > Notifications > Webhooks"},
}

// owns says whether an environment variable is named for the service.
func (sv service) owns(name string) bool { return strings.Contains(name, sv.env) }

// pipNames maps Python imports to the package that provides them, where the two differ.
var pipNames = map[string]string{
	"sklearn": "scikit-learn", "PIL": "pillow", "cv2": "opencv-python", "yaml": "pyyaml",
	"bs4": "beautifulsoup4", "dotenv": "python-dotenv", "dateutil": "python-dateutil",
	"docx": "python-docx", "pptx": "python-pptx", "jwt": "pyjwt", "fitz": "pymupdf",
	"Crypto": "pycryptodome", "MySQLdb": "mysqlclient", "magic": "python-magic",
	"serial": "pyserial", "attr": "attrs", "googleapiclient": "google-api-python-client",
	"streamlit_authenticator": "streamlit-authenticator", "flask_sqlalchemy": "flask-sqlalchemy",
	"flask_login": "flask-login", "flask_cors": "flask-cors", "jose": "python-jose",
	"multipart": "python-multipart",
}

// pythonStdlib is Python's standard library (sys.stdlib_module_names, Python 3.12), minus
// the rarely used.
var pythonStdlib = setOf(`__future__ abc argparse array ast asyncio atexit base64 bisect
	builtins bz2 calendar cgi cmath codecs collections colorsys concurrent configparser
	contextlib contextvars copy csv ctypes dataclasses datetime decimal difflib dis email
	encodings enum errno faulthandler fcntl filecmp fileinput fnmatch fractions ftplib
	functools gc getpass gettext glob graphlib gzip hashlib heapq hmac html http imaplib
	importlib inspect io ipaddress itertools json keyword linecache locale logging lzma
	mailbox math mimetypes mmap multiprocessing netrc numbers operator os pathlib pdb pickle
	pkgutil platform plistlib poplib posixpath pprint profile pstats pty pwd queue quopri
	random re readline reprlib resource runpy sched secrets select selectors shelve shlex
	shutil signal site smtplib socket socketserver sqlite3 ssl stat statistics string
	stringprep struct subprocess sys sysconfig syslog tarfile tempfile termios textwrap
	threading time timeit tkinter token tokenize tomllib trace traceback tracemalloc tty
	turtle types typing unicodedata unittest urllib uuid venv warnings wave weakref
	webbrowser winreg wsgiref xml xmlrpc zipapp zipfile zipimport zlib zoneinfo`)

func setOf(words string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(words) {
		set[w] = true
	}
	return set
}

"""Example: collect a single URL through the proxy using Lightpanda CDP + chromedp-less HTTP.

Proxy creds come from env (never committed). Defaults are placeholders.
"""
import os, requests

username = os.getenv("PROXY_USERNAME", "USERNAME")
password = os.getenv("PROXY_PASSWORD", "PASSWORD")
host = os.getenv("PROXY_HOST", "isp.decodo.com")
port = os.getenv("PROXY_PORT", "10000")
proxy = f"http://{username}:{password}@{host}:{port}" if username not in ("", "USERNAME") else None
proxies = {"http": proxy, "https": proxy} if proxy else None

url = os.getenv("TARGET", "https://example.com")
r = requests.get(url, proxies=proxies, timeout=30)
print(r.status_code, r.text[:500])
"""Fetch a proxy list from the Webshare API and print the first proxy URL.

Env: WEBSHARE_TOKEN
"""
import os, requests

token = os.environ["WEBSHARE_TOKEN"]
url = "https://proxy.webshare.io/api/v2/proxy/list/?mode=direct&page=1&page_size=100"
r = requests.get(url, headers={"Authorization": f"Token {token}"}, timeout=10)
r.raise_for_status()
p = r.json()["results"][0]
print(f"http://{p['username']}:{p['password']}@{p['proxy_address']}:{p['port']}")
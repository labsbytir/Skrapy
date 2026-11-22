"""Bright Data: collect a URL via their Browser API (CDP) or SDK.

Env: BRIGHTDATA_API_KEY (or BRIGHTDATA_API_TOKEN)
"""
import os
from brightdata import SyncBrightDataClient

with SyncBrightDataClient() as client:
    result = client.scrape_url(os.getenv("TARGET", "https://example.com"))
    print(result.data[:500] if result.success else result.error)
import requests

url = 'https://ip.decodo.com/json'
username = 'USERNAME'
password = 'PASSWORD'
proxy = f"http://{username}:{password}@isp.decodo.com:10000"
result = requests.get(url, proxies={
    'http': proxy,
    'https': proxy
})
print(result.text)
import urllib, json
from collections import Counter

url = "https://api.coinmarketcap.com/v1/ticker/"
response = urllib.urlopen(url)
data = json.loads(response.read())

counts = Counter([len(asset['symbol']) for asset in data])
for i, j in sorted(counts.items()):
    print("Asset symbols of length %d: %d" % (i, j))


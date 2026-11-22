const { createWriteStream, chmodSync, mkdirSync, existsSync } = require('fs');
const { get } = require('https');
const { join } = require('path');

const REPO = 'labsbytir/GoNewsk';
const VERSION = require('../package.json').version;
const ASSETS = { linux: 'gonewsk-linux-amd64', darwin: 'gonewsk-macos-arm64' };
const asset = ASSETS[process.platform];
if (!asset) {
  console.error(`gonewsk: unsupported platform ${process.platform}`);
  process.exit(1);
}

const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
if (!token) {
  console.error('gonewsk: set GITHUB_TOKEN to download the binary (repo is private).');
  process.exit(1);
}

const vendor = join(__dirname, '..', 'vendor');
mkdirSync(vendor, { recursive: true });
const dest = join(vendor, 'gonewsk');

function req(url, headers = {}) {
  return new Promise((resolve, reject) => {
    get(url, { headers: { 'User-Agent': 'gonewsk-npm', Authorization: `Bearer ${token}`, ...headers } }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        // strip auth on cross-host redirect
        get(res.headers.location, { headers: { 'User-Agent': 'gonewsk-npm' } }, resolve).on('error', reject);
        return;
      }
      resolve(res);
    }).on('error', reject);
  });
}

(async () => {
  if (existsSync(dest)) return;
  const api = await req(`https://api.github.com/repos/${REPO}/releases/tags/v${VERSION}`);
  let body = '';
  for await (const c of api) body += c;
  const rel = JSON.parse(body);
  const a = rel.assets.find((x) => x.name === asset);
  if (!a) {
    console.error(`gonewsk: no ${asset} in release v${VERSION}`);
    process.exit(1);
  }
  const dl = await req(`https://api.github.com/repos/${REPO}/releases/assets/${a.id}`, { Accept: 'application/octet-stream' });
  await new Promise((res, rej) => {
    dl.pipe(createWriteStream(dest, { mode: 0o755 })).on('finish', res).on('error', rej);
  });
  chmodSync(dest, 0o755);
  console.log(`gonewsk: installed ${dest}`);
})().catch((e) => { console.error('gonewsk:', e.message); process.exit(1); });

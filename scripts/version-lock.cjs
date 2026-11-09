const fs = require('node:fs');
const { execFileSync } = require('node:child_process');
// ppackage supplies the target before rewriting package.json.
const lock = JSON.parse(fs.readFileSync('package-lock.json', 'utf8'));
lock.version = lock.packages[''].version = process.env.npm_package_version;
fs.writeFileSync('package-lock.json', JSON.stringify(lock, null, 2) + '\n');
execFileSync('git', ['add', 'package-lock.json']);

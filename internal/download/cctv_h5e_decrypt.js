#!/usr/bin/env node
/**
 * CCTV H5E TS Segment Decryptor (mediago embedded)
 *
 * Loads CCTV's official WASM module, patches it in-memory for Node.js,
 * and decrypts h5e-encrypted TS segments.
 *
 * Usage:
 *   node --stack-size=65536 cctv_h5e_decrypt.js <worker.js> <input.ts> <output.ts>
 *
 * Batch:
 *   echo '[{"input":"a.ts","output":"b.ts"}]' | \
 *     node --stack-size=65536 cctv_h5e_decrypt.js --batch <worker.js>
 */
'use strict';
const fs = require('fs');

const MEDIA_TAG_ID = 'player_container_player##1000000##0';
const VIDEO_PID = 0x100;
const PAGE_HOST = 'tv.cctv.com';
const INIT_DELAY_MS = 1500;

const H5PLAYER_JSON = '{"h5player":{"ver":20190904,"md5":"c7ed5a71dbe4dee1a2ba171f660ee98d","BTime":"2019-09-04-20:25:10"}}';
const H5PLAYER_B64 = 'eyJoNXBsYXllciI6eyJ2ZXIiOjIwMTkwOTA0LCJtZDUiOiJjN2VkNWE3MWRiZTRkZWUxYTJiYTE3MWY2NjBlZTk4ZCIsIkJUaW1lIjoiMjAxOS0wOS0wNC0yMDoyNToxMCJ9fQ==';

// Blob-worker context (host must be empty to match WASM VMP expectations)
global.self = { location: { host: '', hostname: '', href: 'blob:https://tv.cctv.com/5bca710b-9f02-41f0-a9f1-102bbc65192a', origin: 'https://tv.cctv.com', protocol: 'blob:', pathname: '', port: '', search: '', hash: '' } };
global.location = global.self.location;
global.document = { currentScript: { src: '' } };
Object.defineProperty(global, 'navigator', { value: { userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36' }, writable: true, configurable: true });
global.fetch = (url) => {
    if (url && typeof url === 'string' && url.includes('H5player')) {
        const buf = Buffer.from(H5PLAYER_JSON);
        return Promise.resolve({ ok: true, status: 200, statusText: 'OK', arrayBuffer: () => Promise.resolve(buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength)) });
    }
    return Promise.resolve({ ok: true, status: 200, statusText: 'OK', arrayBuffer: () => Promise.resolve(new ArrayBuffer(0)) });
};

function patchWorkerCode(code) {
    // 1. theAnswer: replace eval(name) with eval("i."+name) using fake blob location
    const fakeLocation = '({hash:"",host:"",hostname:"",href:"blob:https://tv.cctv.com/5bca710b-9f02-41f0-a9f1-102bbc65192a",origin:"https://tv.cctv.com",pathname:"",port:"",protocol:"blob:",search:""})';
    code = code.replace(
        'theAnswer(thearg){var name=UTF8ToString(thearg),a=eval(name)',
        `theAnswer(thearg){var i={location:${fakeLocation}};i.self={location:i.location};var name=UTF8ToString(thearg),a=eval("i."+name)`
    );

    // 2. emval_get_global: return fake object instead of globalThis
    code = code.replace(
        /function emval_get_global\(\)\{return"object"==typeof globalThis\?globalThis:Function\("return this"\)\(\)\}/,
        `function emval_get_global(){var i={location:${fakeLocation}};i.self={location:i.location};return i}`
    );

    // 3. _emscripten_get_callstack: return fake blob URL (anti-debug bypass)
    code = code.replace(
        'A=_emscripten_get_callstack_js(A)',
        'A="blob:https://tv.cctv.com/5bca710b-9f02-41f0-a9f1-102bbc65192a"'
    );

    // 4. XMLHttpRequest → privateXMLHttpRequest with H5player.json intercept
    const xhrClass = `class privateXMLHttpRequest{constructor(){this.status=0;this.readyState=0;this.response=null;this.responseText="";this.statusText="";this.responseType="";this.timeout=0;this.withCredentials=false;this.url_=""}open(m,u,a){this.url_=u||"";if(this.url_==="https://tv.cctv.com/Library/H5player.json")this.url_="data:application/json;base64,${H5PLAYER_B64}";this.readyState=1;if(this.onreadystatechange)this.onreadystatechange()}setRequestHeader(){}overrideMimeType(){}getResponseHeader(){return null}send(body){const url=this.url_;if(url&&url.startsWith("data:")){const c=url.indexOf(","),meta=url.slice(5,c),pl=url.slice(c+1);if(meta.includes("base64")){const buf=Buffer.from(pl,"base64");if(this.responseType==="arraybuffer"){this.response=buf.buffer.slice(buf.byteOffset,buf.byteOffset+buf.byteLength)}else{this.responseText=buf.toString("utf-8");this.response=this.responseText}}this.status=200;this.readyState=4;if(this.onload)this.onload();if(this.onreadystatechange)this.onreadystatechange();return}if(typeof fetch!=="undefined"){fetch(url,{body,method:"GET"}).then(r=>{this.readyState=4;this.status=r.status;this.statusText=r.statusText||"";if(!r.ok)throw new Error;return r.arrayBuffer()}).then(ab=>{this.response=ab;if(this.onload)this.onload();if(this.onreadystatechange)this.onreadystatechange()}).catch(()=>{if(this.onerror)this.onerror();if(this.onreadystatechange)this.onreadystatechange()})}}}\n`;
    code = code.replace('return function(CNTVModule)', xhrClass + 'return function(CNTVModule)');
    code = code.replace(/new XMLHttpRequest/g, 'new privateXMLHttpRequest');
    code = code.replace(/"undefined"!=typeof XMLHttpRequest/g, '"undefined"!=typeof privateXMLHttpRequest');

    // 5. Fetch short-circuit (variable names differ between worker versions)
    code = code.replace(/([a-z])=!!\(4&([a-z])\),([a-z])=!!\(32&\2\),\2=!!\(16&\2\);/, '$1=!!(4&$2),$3=!!(32&$2),$2=!!(16&$2);__emscripten_fetch_xhr(A,n,E,I,o);return A;');

    return code;
}

function loadAndInit(workerPath) {
    return new Promise((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error('WASM init timeout (30s)')), 30000);
        let code = fs.readFileSync(workerPath, 'utf-8');
        code = patchWorkerCode(code);
        (0, eval)(code);

        if (typeof CNTVModule !== 'function') {
            clearTimeout(timeout);
            return reject(new Error('CNTVModule not found after loading worker'));
        }

        const Module = CNTVModule({
            onRuntimeInitialized() {
                clearTimeout(timeout);
                const tagBuf = Buffer.from(MEDIA_TAG_ID + '\0');
                const tagAddr = Module._malloc(tagBuf.length);
                Module.HEAPU8.set(tagBuf, tagAddr);
                Module._CNTV_InitPlayer(tagAddr);

                // Wait for async H5player.json fetch to complete
                setTimeout(() => resolve({ Module, tagAddr, tagBuf }), INIT_DELAY_MS);
            }
        });
    });
}

function decryptSegment(Module, tagAddr, tagBuf, tsData) {
    const data = Buffer.from(tsData);
    const hostBuf = Buffer.from(PAGE_HOST + '\0');
    const pesUnits = collectPES(data);
    let shouldDecrypt = true, decCount = 0;

    for (const pes of pesUnits) {
        const pesData = Buffer.concat(pes.bufs);
        if (pesData.length < 9 || pesData[0] !== 0 || pesData[1] !== 0 || pesData[2] !== 1) continue;
        const hdrLen = pesData[8];
        const esOff = 9 + hdrLen;
        const es = Buffer.from(pesData.slice(esOff));
        const nals = findNALs(es);
        let modified = false;

        for (const { start, end, type } of nals) {
            if (type === 25) {
                if (end - start > 1) shouldDecrypt = es[start + 1] === 1;
                continue;
            }
            if ((type === 1 || type === 5) && shouldDecrypt) {
                const nalu = es.slice(start, end);
                if (nalu.length < 33) continue;

                const vmpTag = (Module._CNTV_UpdatePlayer(tagAddr) >>> 0).toString(16).padStart(8, '0');
                const dAddr = Module._malloc(nalu.length + 1 + hostBuf.length);
                Module.HEAPU8.set(nalu, dAddr);
                Module.HEAPU8[dAddr + nalu.length] = 0;
                Module.HEAPU8.set(hostBuf, dAddr + nalu.length + 1);

                const t2 = Module._malloc(tagBuf.length);
                Module.HEAPU8.set(tagBuf, t2);

                for (let i = 0; i < 8; i++) {
                    if ('0123456'.includes(vmpTag[i])) {
                        const fn = Module[`_CNTV_jsdecVOD${7 - i}`];
                        if (fn) fn(t2, dAddr, nalu.length + 1, hostBuf.length);
                    }
                }
                const decLen = Module._CNTV_jsdecVOD8(t2, dAddr, nalu.length + 1, hostBuf.length);
                const outLen = decLen > 0 ? Math.min(decLen, end - start) : nalu.length;
                const dec = Buffer.from(Module.HEAPU8.subarray(dAddr, dAddr + outLen));
                dec.copy(es, start, 0, Math.min(outLen, end - start));
                modified = true;
                decCount++;

                Module._free(dAddr);
                Module._free(t2);
            }
        }

        if (!modified) continue;
        const newPES = Buffer.concat([pesData.slice(0, esOff), es]);
        let off = 0;
        for (const { pos, ps } of pes.pkts) {
            const slot = pos + 188 - ps;
            newPES.copy(data, ps, off, off + slot);
            off += slot;
        }
    }
    return { data, decCount };
}

function collectPES(data) {
    const units = [];
    let cur = null;
    for (let pos = 0; pos < data.length; pos += 188) {
        if (data[pos] !== 0x47) continue;
        const pid = ((data[pos + 1] & 0x1f) << 8) | data[pos + 2];
        if (pid !== VIDEO_PID) continue;
        const pusi = (data[pos + 1] & 0x40) >> 6;
        const afc = (data[pos + 3] & 0x30) >> 4;
        let ps;
        if (afc === 1) ps = pos + 4;
        else if (afc === 3) ps = pos + 5 + data[pos + 4];
        else continue;
        if (pusi) {
            if (cur) units.push(cur);
            cur = { bufs: [data.slice(ps, pos + 188)], pkts: [{ pos, ps }] };
        } else if (cur) {
            cur.bufs.push(data.slice(ps, pos + 188));
            cur.pkts.push({ pos, ps });
        }
    }
    if (cur) units.push(cur);
    return units;
}

function findNALs(es) {
    const result = [];
    const starts = [];
    for (let i = 0; i < es.length - 3; i++) {
        if (es[i] === 0 && es[i + 1] === 0) {
            if (i + 3 < es.length && es[i + 2] === 0 && es[i + 3] === 1) { starts.push({ sc: i, s: i + 4 }); i += 3; }
            else if (es[i + 2] === 1) { starts.push({ sc: i, s: i + 3 }); i += 2; }
        }
    }
    for (let i = 0; i < starts.length; i++) {
        const start = starts[i].s;
        const end = i + 1 < starts.length ? starts[i + 1].sc : es.length;
        result.push({ start, end, type: es[start] & 0x1f });
    }
    return result;
}

async function main() {
    const args = process.argv.slice(2);
    let workerPath, jobs;

    if (args[0] === '--batch') {
        workerPath = args[1];
        jobs = JSON.parse(fs.readFileSync(0, 'utf-8'));
    } else {
        workerPath = args[0];
        jobs = [{ input: args[1], output: args[2] }];
    }

    if (!workerPath || !jobs[0] || !jobs[0].input) {
        process.stderr.write('Usage: node --stack-size=65536 cctv_h5e_decrypt.js <worker.js> <input.ts> <output.ts>\n');
        process.exit(1);
    }

    const { Module, tagAddr, tagBuf } = await loadAndInit(workerPath);

    for (const job of jobs) {
        try {
            const ts = fs.readFileSync(job.input);
            const { data, decCount } = decryptSegment(Module, tagAddr, tagBuf, ts);
            fs.writeFileSync(job.output, data);
            process.stdout.write(JSON.stringify({ ok: true, file: job.output, size: data.length, nalu_count: decCount }) + '\n');
        } catch (e) {
            process.stdout.write(JSON.stringify({ ok: false, file: job.output, error: e.message }) + '\n');
        }
    }
}

main().catch(e => { process.stderr.write(e.message + '\n'); process.exit(1); });

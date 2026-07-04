#!/usr/bin/env node
/**
 * CCTV H5E TS Segment Decryptor (mediago embedded)
 *
 * Pure-JS TEA decryptor. No WASM, no worker.js, no --stack-size, no deps.
 *
 * Usage:
 *   node cctv_h5e_decrypt.js <input.ts> <output.ts>
 *
 * Batch (stdin JSON array of {input,output}):
 *   echo '[{"input":"a.ts","output":"b.ts"}]' | node cctv_h5e_decrypt.js --batch
 *
 * Emits one JSON line per segment: {"ok":true,"file":...,"size":N,"nalu_count":M}
 */
'use strict';
const fs = require('fs');

const VIDEO_PID = 0x100;
const TEA_DELTA = 0x9e3779b9;

// TEA decrypt one 8-byte block (two u32 LE words). 16 rounds, sum starts at delta*16.
function teaDecryptBlock(v0, v1, k0, k1, k2, k3) {
    let sum = (TEA_DELTA * 16) >>> 0;
    for (let i = 0; i < 16; i++) {
        v1 = (v1 - ((((v0 << 4) >>> 0) + k2 ^ (v0 + sum) ^ ((v0 >>> 5) + k3)) >>> 0)) >>> 0;
        v0 = (v0 - ((((v1 << 4) >>> 0) + k0 ^ (v1 + sum) ^ ((v1 >>> 5) + k1)) >>> 0)) >>> 0;
        sum = (sum - TEA_DELTA) >>> 0;
    }
    return [v0 >>> 0, v1 >>> 0];
}

// Strip H.264 emulation-prevention bytes (00 00 03 -> 00 00) from a NAL unit.
function removeSCEP(nal) {
    const out = Buffer.allocUnsafe(nal.length);
    let j = 0, i = 0;
    const n = nal.length;
    while (i < n) {
        if (i + 2 < n && nal[i] === 0 && nal[i + 1] === 0 && nal[i + 2] === 3) {
            out[j++] = 0;
            out[j++] = 0;
            i += 3;
        } else {
            out[j++] = nal[i++];
        }
    }
    return out.subarray(0, j);
}

// TEA-decrypt an SCEP-removed NAL payload in place.
// Key = bytes 16..31 (4 u32 LE). Decrypt 8 bytes every 80 bytes starting at offset 32.
function teaDecryptNAL(clean) {
    if (clean.length < 33) return;
    const k0 = clean.readUInt32LE(16);
    const k1 = clean.readUInt32LE(20);
    const k2 = clean.readUInt32LE(24);
    const k3 = clean.readUInt32LE(28);
    const iterations = Math.floor((clean.length - 32) / 80);
    for (let i = 0; i < iterations; i++) {
        const off = 32 + i * 80;
        const v0 = clean.readUInt32LE(off);
        const v1 = clean.readUInt32LE(off + 4);
        const [d0, d1] = teaDecryptBlock(v0, v1, k0, k1, k2, k3);
        clean.writeUInt32LE(d0, off);
        clean.writeUInt32LE(d1, off + 4);
    }
}

// Locate NAL units in an ES buffer.
// Returns { scStart, scLen, nalStart, nalEnd } where nalEnd is the next start-code position.
function parseNALs(es) {
    const n = es.length;
    const units = [];
    let i = 0;
    while (i < n - 3) {
        if (es[i] === 0 && es[i + 1] === 0) {
            if (i + 3 < n && es[i + 2] === 0 && es[i + 3] === 1) {
                units.push({ scStart: i, scLen: 4, nalStart: i + 4 });
                i += 4;
            } else if (es[i + 2] === 1) {
                units.push({ scStart: i, scLen: 3, nalStart: i + 3 });
                i += 3;
            } else {
                i++;
            }
        } else {
            i++;
        }
    }
    const result = [];
    for (let k = 0; k < units.length; k++) {
        const nalEnd = k + 1 < units.length ? units[k + 1].scStart : n;
        result.push({ ...units[k], nalEnd });
    }
    return result;
}

// Decrypt one ES buffer. Removes SCEP from type 1/5/25 NALs, TEA-decrypts, and
// rebuilds the ES WITHOUT re-inserting SCEP bytes (output is shorter).
// Returns { es: Buffer, count } or null if nothing was decrypted.
function decryptES(es) {
    const nals = parseNALs(es);
    if (nals.length === 0) return null;

    const parts = [];
    let prevEnd = 0;
    let count = 0;

    for (const { scStart, scLen, nalStart, nalEnd } of nals) {
        if (scStart > prevEnd) parts.push(es.subarray(prevEnd, scStart));
        const startCode = es.subarray(scStart, scStart + scLen);
        const nal = es.subarray(nalStart, nalEnd);
        const type = nal.length ? nal[0] & 0x1f : 0;

        if (type === 1 || type === 5 || type === 25) {
            const clean = removeSCEP(nal);
            teaDecryptNAL(clean);
            parts.push(startCode);
            parts.push(clean); // shorter (SCEP removed), NOT re-added
            count++;
        } else {
            parts.push(startCode);
            parts.push(nal);
        }
        prevEnd = nalEnd;
    }
    if (prevEnd < es.length) parts.push(es.subarray(prevEnd));

    if (count === 0) return null;
    return { es: Buffer.concat(parts), count };
}

// Collect video-PID PES units and the TS packet payload slots that carry them.
function collectPES(data) {
    const units = [];
    let cur = null;
    for (let pos = 0; pos < data.length; pos += 188) {
        if (data[pos] !== 0x47) continue;
        const pid = ((data[pos + 1] & 0x1f) << 8) | data[pos + 2];
        if (pid !== VIDEO_PID) continue;
        const pusi = (data[pos + 1] & 0x40) >> 6;
        const afc = (data[pos + 3] & 0x30) >> 4;
        if (afc === 0 || afc === 2) continue;
        const ps = afc === 3 ? pos + 5 + data[pos + 4] : pos + 4;
        const pe = pos + 188;
        if (pusi) {
            if (cur) units.push(cur);
            cur = { bufs: [data.subarray(ps, pe)], slots: [{ ps, pe }] };
        } else if (cur) {
            cur.bufs.push(data.subarray(ps, pe));
            cur.slots.push({ ps, pe });
        }
    }
    if (cur) units.push(cur);
    return units;
}

// Decrypt a full TS segment. Returns { data: Buffer, count }.
function decryptSegment(tsData) {
    const data = Buffer.from(tsData);
    const pesUnits = collectPES(data);
    let count = 0;

    for (const pes of pesUnits) {
        const pesData = Buffer.concat(pes.bufs);
        if (pesData.length < 9 || pesData[0] !== 0 || pesData[1] !== 0 || pesData[2] !== 1) continue;
        const hdrLen = pesData[8];
        const esOff = 9 + hdrLen;
        const es = pesData.subarray(esOff);

        const res = decryptES(es);
        if (!res) continue;
        count += res.count;

        // Rebuilt PES is shorter than the original ES (SCEP bytes dropped).
        // Scatter it back across the original TS payload slots, padding the tail
        // of each slot with 0xFF stuffing when the cleaned data runs out.
        const newPES = Buffer.concat([pesData.subarray(0, esOff), res.es]);
        let off = 0;
        for (const { ps, pe } of pes.slots) {
            const slot = pe - ps;
            const chunk = newPES.subarray(off, off + slot);
            if (chunk.length > 0) chunk.copy(data, ps);
            if (chunk.length < slot) data.fill(0xff, ps + chunk.length, pe);
            off += slot;
        }
    }
    return { data, count };
}

function runJob(job) {
    try {
        const ts = fs.readFileSync(job.input);
        const { data, count } = decryptSegment(ts);
        fs.writeFileSync(job.output, data);
        process.stdout.write(JSON.stringify({ ok: true, file: job.output, size: data.length, nalu_count: count }) + '\n');
    } catch (e) {
        process.stdout.write(JSON.stringify({ ok: false, file: job.output, error: e.message }) + '\n');
    }
}

function main() {
    const args = process.argv.slice(2);
    let jobs;

    if (args[0] === '--batch') {
        jobs = JSON.parse(fs.readFileSync(0, 'utf-8'));
    } else {
        if (!args[0] || !args[1]) {
            process.stderr.write('Usage: node cctv_h5e_decrypt.js <input.ts> <output.ts>\n');
            process.stderr.write('Batch: echo \'[{"input":"a.ts","output":"b.ts"}]\' | node cctv_h5e_decrypt.js --batch\n');
            process.exit(1);
        }
        jobs = [{ input: args[0], output: args[1] }];
    }

    for (const job of jobs) runJob(job);
}

main();

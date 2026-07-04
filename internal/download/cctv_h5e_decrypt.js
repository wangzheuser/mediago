global.self={location:{host:'',hostname:'',href:'blob:https://tv.cctv.com/5bca710b',origin:'https://tv.cctv.com',protocol:'blob:'}};
global.location=global.self.location;global.document={currentScript:{src:''}};
Object.defineProperty(global,'navigator',{value:{userAgent:'x'},writable:true,configurable:true});
global.fetch=()=>Promise.resolve({ok:true,status:200,statusText:'OK',arrayBuffer:()=>Promise.resolve(Buffer.from('{"h5player":{"ver":20190904}}').buffer)});
const fs=require('fs');
const workerPath=process.argv[2];
const jobsJSON=fs.readFileSync(0,'utf-8');
const jobs=JSON.parse(jobsJSON);

// Load and patch worker (skip if already patched)
let code=fs.readFileSync(workerPath,'utf-8');
if (!code.includes('privateXMLHttpRequest')) {
// patches inline...
code=code.replace('theAnswer(thearg){var name=UTF8ToString(thearg),a=eval(name','theAnswer(thearg){var i={location:{hash:"",host:"",hostname:"",href:"blob:https://tv.cctv.com/5bca710b-9f02-41f0-a9f1-102bbc65192a",origin:"https://tv.cctv.com",pathname:"",port:"",protocol:"blob:",search:""}};i.self={location:i.location};var name=UTF8ToString(thearg),a=eval("i."+name');
code=code.replace(/function emval_get_global\(\)\{return"object"==typeof globalThis\?globalThis:Function\("return this"\)\(\)\}/,'function emval_get_global(){var i={location:{hash:"",host:"",hostname:"",href:"blob:https://tv.cctv.com/5bca710b",origin:"https://tv.cctv.com",pathname:"",port:"",protocol:"blob:",search:""}};i.self={location:i.location};return i}');
code=code.replace('A=_emscripten_get_callstack_js(A)','A="blob:https://tv.cctv.com/5bca710b"');
const xhr='class privateXMLHttpRequest{constructor(){this.status=0;this.readyState=0;this.response=null;this.responseText="";this.statusText="";this.responseType="";this.timeout=0;this.withCredentials=false;this.url_=""}open(m,u,a){this.url_=u||"";if(this.url_==="https://tv.cctv.com/Library/H5player.json")this.url_="data:application/json;base64,eyJoNXBsYXllciI6eyJ2ZXIiOjIwMTkwOTA0LCJtZDUiOiJjN2VkNWE3MWRiZTRkZWUxYTJiYTE3MWY2NjBlZTk4ZCIsIkJUaW1lIjoiMjAxOS0wOS0wNC0yMDoyNToxMCJ9fQ==";this.readyState=1;if(this.onreadystatechange)this.onreadystatechange()}setRequestHeader(){}overrideMimeType(){}getResponseHeader(){return null}send(body){const url=this.url_;if(url&&url.startsWith("data:")){const c=url.indexOf(","),meta=url.slice(5,c),pl=url.slice(c+1);if(meta.includes("base64")){const buf=Buffer.from(pl,"base64");if(this.responseType==="arraybuffer")this.response=buf.buffer.slice(buf.byteOffset,buf.byteOffset+buf.byteLength);else{this.responseText=buf.toString("utf-8");this.response=this.responseText}}this.status=200;this.readyState=4;if(this.onload)this.onload();if(this.onreadystatechange)this.onreadystatechange();return}if(typeof fetch!=="undefined"){fetch(url,{body,method:"GET"}).then(r=>{this.readyState=4;this.status=r.status;if(!r.ok)throw new Error;return r.arrayBuffer()}).then(ab=>{this.response=ab;if(this.onload)this.onload();if(this.onreadystatechange)this.onreadystatechange()}).catch(()=>{if(this.onerror)this.onerror();if(this.onreadystatechange)this.onreadystatechange()})}}}\n';
code=code.replace('return function(CNTVModule)',xhr+'return function(CNTVModule)');
code=code.replace(/new XMLHttpRequest/g,'new privateXMLHttpRequest');
code=code.replace(/"undefined"!=typeof XMLHttpRequest/g,'"undefined"!=typeof privateXMLHttpRequest');
code=code.replace(/([a-z])=!!\(4&([a-z])\),([a-z])=!!\(32&\2\),\2=!!\(16&\2\);/,'$1=!!(4&$2),$3=!!(32&$2),$2=!!(16&$2);__emscripten_fetch_xhr(A,n,E,I,o);return A;');
} // end if (!code.includes('privateXMLHttpRequest'))

(0,eval)(code);

const TAG='player_container_player',HOST='https://tv.cctv.com',EXT=2048,HB=Array.from(HOST,c=>c.charCodeAt(0));
const M=CNTVModule({onRuntimeInitialized(){
    const tm=M._jsmalloc(TAG.length+EXT);M.HEAP8.fill(0,tm,tm+TAG.length+EXT);M.HEAP8.set(Array.from(TAG,c=>c.charCodeAt(0)),tm);
    M._CNTV_InitPlayer(tm);
    const t2s=M._jsmalloc((TAG+'##1000000##0').length+1);M.HEAP8.set(Array.from(TAG+'##1000000##0',c=>c.charCodeAt(0)),t2s);
    const t2n=M._jsmalloc(TAG.length+1);M.HEAP8.set(Array.from(TAG,c=>c.charCodeAt(0)),t2n);
    const db=M._jsmalloc(512*1024);
    const t0=Date.now();
    for(let ji=0;ji<jobs.length;ji++){const job=jobs[ji];try{
        const ts=Buffer.from(fs.readFileSync(job.input));
        let cur=null;const pu=[];
        for(let p=0;p<ts.length;p+=188){if(ts[p]!==0x47)continue;const pid=((ts[p+1]&0x1f)<<8)|ts[p+2];if(pid!==0x100)continue;const pusi=(ts[p+1]&0x40)>>6,afc=(ts[p+3]&0x30)>>4;let ps=afc===1?p+4:afc===3?p+5+ts[p+4]:-1;if(ps<0)continue;if(pusi){if(cur)pu.push(cur);cur={bufs:[ts.slice(ps,p+188)],pkts:[{pos:p,ps}]};}else if(cur){cur.bufs.push(ts.slice(ps,p+188));cur.pkts.push({pos:p,ps});}}
        if(cur)pu.push(cur);let dc=0,sd=true;
        for(const pes of pu){const pd=Buffer.concat(pes.bufs);if(pd.length<9||pd[0]!==0||pd[1]!==0||pd[2]!==1)continue;const hl=pd[8],eo=9+hl,es=Buffer.from(pd.slice(eo));const nals=[];for(let i=0;i<es.length-3;i++){if(es[i]===0&&es[i+1]===0){if(i+3<es.length&&es[i+2]===0&&es[i+3]===1){nals.push({sc:i,s:i+4});i+=3;}else if(es[i+2]===1){nals.push({sc:i,s:i+3});i+=2;}}}let mod=false;
        for(let ni=0;ni<nals.length;ni++){const s=nals[ni].s,e=ni+1<nals.length?nals[ni+1].sc:es.length;const t=es[s]&0x1f;
            if(t===25){if(e-s>1)sd=es[s+1]===1;const pay=es.slice(s+1,e);M.HEAP8[db]=es[s];M.HEAP8.set(pay,db+1);M.HEAP8.set(HB,db+pay.length+1);const vt=(M._CNTV_UpdatePlayer(tm)>>>0).toString(16).padStart(8,'0');for(let i=0;i<8;i++){if('0123456'.includes(vt[i])){const fn=M['_CNTV_jsdecVOD'+(7-i)];if(fn)fn(t2n,db,pay.length+1,HOST.length);}}M._CNTV_jsdecVOD8(t2n,db,pay.length+1,HOST.length);continue;}
            if((t===1||t===5)&&sd){const pay=es.slice(s+1,e);M.HEAP8[db]=es[s];M.HEAP8.set(pay,db+1);M.HEAP8.set(HB,db+pay.length+1);const vt=(M._CNTV_UpdatePlayer(tm)>>>0).toString(16).padStart(8,'0');for(let i=0;i<8;i++){if('0123456'.includes(vt[i])){const fn=M['_CNTV_jsdecVOD'+(7-i)];if(fn)fn(t2s,db,pay.length+1,HOST.length);}}const dl=M._CNTV_jsdecVOD8(t2s,db,pay.length+1,HOST.length);const dec=Buffer.from(M.HEAP8.slice(db,db+dl));dec.slice(0,Math.min(dl,e-s)).copy(es,s);mod=true;dc++;}}
        if(!mod)continue;const np=Buffer.concat([pd.slice(0,eo),es]);let off=0;for(const{pos,ps}of pes.pkts){const sl=pos+188-ps;np.copy(ts,ps,off,off+sl);off+=sl;}}
        fs.writeFileSync(job.output,ts);process.stdout.write(JSON.stringify({ok:true,file:job.output,size:ts.length,nalu_count:dc})+'\n');
    }catch(e){process.stdout.write(JSON.stringify({ok:false,file:job.output,error:e.message})+'\n');}}
    process.stderr.write('[decrypt] done: '+jobs.length+' segments in '+((Date.now()-t0)/1000).toFixed(1)+'s\n');
    process.exit(0);
}});
setTimeout(()=>{process.stderr.write('timeout\n');process.exit(1);},600000);

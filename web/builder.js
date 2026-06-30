"use strict";
const uid=(()=>{let i=0;return()=>"u"+(++i)+Math.random().toString(36).slice(2,6);})();

/* ---- field type taxonomy ---- */
const TYPES={input:["text","textarea","number","integer","decimal","currency","range","rating","calculated","hidden"],
  choice:["select","multiselect","radio","checkbox","boolean"],time:["date","time","datetime"],
  media:["geopoint","photo","file","signature","barcode"],struct:["markdown","note"]};
const CAT_OF={}; Object.entries(TYPES).forEach(([c,a])=>a.forEach(t=>CAT_OF[t]=c));
const CAT_VAR={input:"--input",choice:"--choice",time:"--time",media:"--media",struct:"--struct",node:"--node"};
const LABELS={text:"Teks singkat",textarea:"Teks panjang",number:"Angka",integer:"Bilangan bulat",decimal:"Desimal",currency:"Mata uang",range:"Slider",rating:"Rating",calculated:"Terhitung",hidden:"Tersembunyi",select:"Dropdown",multiselect:"Pilih banyak",radio:"Radio",checkbox:"Checkbox",boolean:"Ya/Tidak",date:"Tanggal",time:"Jam",datetime:"Tanggal+jam",geopoint:"Titik GPS",photo:"Foto",file:"Berkas",signature:"Tanda tangan",barcode:"Barcode",note:"Catatan (HTML)",markdown:"Keterangan (Markdown)"};
const CHOICE=new Set(["select","multiselect","radio","checkbox"]);
const NUMERIC=new Set(["number","integer","decimal","currency","range","rating"]);
const TEXTY=new Set(["text","textarea"]);

/* ===================== EXPRESSION ENGINE ===================== */
const Expr=(function(){
  const FUNCS=new Set(["isempty","notempty","len","count","sum","avg","min","max","in","today","age","regex","if","number","round","abs","floor","ceil","contains","upper","lower","trim"]);
  function tokenize(src){
    const t=[];let i=0;const n=src.length;const id=c=>/[A-Za-z0-9_]/.test(c);
    while(i<n){let c=src[i];
      if(c===" "||c==="\t"||c==="\n"||c==="\r"){i++;continue;}
      if(c==="$"&&src[i+1]==="{"){let j=i+2,s="";while(j<n&&src[j]!=="}")s+=src[j++];if(src[j]!=="}")throw new Error("${ } tidak ditutup");i=j+1;t.push({t:"ref",v:s.trim()});continue;}
      if(c==="'"||c==='"'){const q=c;let j=i+1,s="";while(j<n&&src[j]!==q){if(src[j]==="\\"){s+=src[j+1];j+=2;}else s+=src[j++];}if(src[j]!==q)throw new Error("teks tidak ditutup");i=j+1;t.push({t:"str",v:s});continue;}
      if((c>="0"&&c<="9")||(c==="."&&src[i+1]>="0"&&src[i+1]<="9")){let j=i,s="";while(j<n&&((src[j]>="0"&&src[j]<="9")||src[j]===".")){s+=src[j++];}i=j;t.push({t:"num",v:parseFloat(s)});continue;}
      if(/[A-Za-z_]/.test(c)){let j=i,s="";while(j<n&&id(src[j]))s+=src[j++];i=j;
        if(s==="true")t.push({t:"bool",v:true});else if(s==="false")t.push({t:"bool",v:false});else if(s==="null")t.push({t:"null"});else t.push({t:"id",v:s});continue;}
      const two=src.substr(i,2);
      if(["==","!=","<=",">=","&&","||"].includes(two)){t.push({t:"op",v:two});i+=2;continue;}
      if("+-*/%<>!".includes(c)){t.push({t:"op",v:c});i++;continue;}
      if(c==="("){t.push({t:"lp"});i++;continue;}
      if(c===")"){t.push({t:"rp"});i++;continue;}
      if(c==="["){t.push({t:"lb"});i++;continue;}
      if(c==="]"){t.push({t:"rb"});i++;continue;}
      if(c===","){t.push({t:"comma"});i++;continue;}
      throw new Error("karakter tak dikenal: "+c);
    }
    return t;
  }
  function parse(src){
    const toks=tokenize(src);let p=0;
    const peek=()=>toks[p],next=()=>toks[p++];
    const expect=k=>{if(!toks[p]||toks[p].t!==k)throw new Error("diharapkan '"+k+"'");return toks[p++];};
    function or(){let l=and();while(peek()&&peek().t==="op"&&peek().v==="||"){next();l={type:"bin",op:"||",l,r:and()};}return l;}
    function and(){let l=eq();while(peek()&&peek().t==="op"&&peek().v==="&&"){next();l={type:"bin",op:"&&",l,r:eq()};}return l;}
    function eq(){let l=cmp();while(peek()&&peek().t==="op"&&(peek().v==="=="||peek().v==="!=")){const o=next().v;l={type:"bin",op:o,l,r:cmp()};}return l;}
    function cmp(){let l=add();while(peek()&&peek().t==="op"&&["<","<=",">",">="].includes(peek().v)){const o=next().v;l={type:"bin",op:o,l,r:add()};}return l;}
    function add(){let l=mul();while(peek()&&peek().t==="op"&&(peek().v==="+"||peek().v==="-")){const o=next().v;l={type:"bin",op:o,l,r:mul()};}return l;}
    function mul(){let l=un();while(peek()&&peek().t==="op"&&["*","/","%"].includes(peek().v)){const o=next().v;l={type:"bin",op:o,l,r:un()};}return l;}
    function un(){if(peek()&&peek().t==="op"&&(peek().v==="!"||peek().v==="-")){const o=next().v;return {type:"un",op:o,e:un()};}return prim();}
    function prim(){const k=peek();if(!k)throw new Error("ekspresi terpotong");
      if(k.t==="num"||k.t==="str"||k.t==="bool"){next();return {type:"lit",v:k.v};}
      if(k.t==="null"){next();return {type:"lit",v:null};}
      if(k.t==="ref"){next();return {type:"ref",name:k.v};}
      if(k.t==="lp"){next();const e=or();expect("rp");return e;}
      if(k.t==="lb"){next();const items=[];if(peek()&&peek().t!=="rb"){items.push(or());while(peek()&&peek().t==="comma"){next();items.push(or());}}expect("rb");return {type:"arr",items};}
      if(k.t==="id"){next();
        if(peek()&&peek().t==="lp"){const fn=k.v.toLowerCase();if(!FUNCS.has(fn))throw new Error("fungsi tak dikenal: "+k.v);next();const args=[];if(peek()&&peek().t!=="rp"){args.push(or());while(peek()&&peek().t==="comma"){next();args.push(or());}}expect("rp");return {type:"call",fn,args};}
        throw new Error("pakai ${...} untuk merujuk field, bukan '"+k.v+"'");}
      throw new Error("token tak terduga");
    }
    const ast=or();if(p<toks.length)throw new Error("ada token sisa di akhir");return ast;
  }
  const toNum=v=>v===true?1:v===false?0:(v==null||v==="")?NaN:Number(v);
  const isEmp=x=>x==null||x===""||(Array.isArray(x)&&x.length===0);
  const flat=a=>{const o=[];a.forEach(x=>Array.isArray(x)?x.forEach(y=>o.push(y)):o.push(x));return o;};
  const nums=a=>flat(a).map(Number).filter(x=>!isNaN(x));
  const LIB={
    isempty:a=>isEmp(a[0]),notempty:a=>!isEmp(a[0]),
    len:a=>{const x=a[0];return Array.isArray(x)?x.length:(x==null?0:String(x).length);},
    count:a=>{const arr=Array.isArray(a[0])?a[0]:a;return arr.filter(x=>!isEmp(x)).length;},
    sum:a=>flat(a).reduce((s,x)=>s+(isEmp(x)?0:(Number(x)||0)),0),
    avg:a=>{const f=nums(a);return f.length?f.reduce((s,x)=>s+x,0)/f.length:0;},
    min:a=>{const f=nums(a);return f.length?Math.min(...f):0;},
    max:a=>{const f=nums(a);return f.length?Math.max(...f):0;},
    in:a=>{const x=a[0];const rest=a.length===2&&Array.isArray(a[1])?a[1]:a.slice(1);return rest.map(String).includes(String(x));},
    today:()=>new Date().toISOString().slice(0,10),
    age:a=>{const d=new Date(a[0]);if(isNaN(d))return NaN;const t=new Date();let g=t.getFullYear()-d.getFullYear();const m=t.getMonth()-d.getMonth();if(m<0||(m===0&&t.getDate()<d.getDate()))g--;return g;},
    regex:a=>{try{return new RegExp(a[1]).test(String(a[0]??""));}catch(_){return false;}},
    if:a=>a[0]?a[1]:a[2],
    number:a=>{const x=Number(a[0]);return isNaN(x)?0:x;},
    round:a=>{const f=Math.pow(10,a[1]||0);return Math.round((Number(a[0])||0)*f)/f;},
    abs:a=>Math.abs(Number(a[0])||0),floor:a=>Math.floor(Number(a[0])||0),ceil:a=>Math.ceil(Number(a[0])||0),
    contains:a=>{const h=a[0];return Array.isArray(h)?h.map(String).includes(String(a[1])):String(h??"").includes(String(a[1]));},
    upper:a=>String(a[0]??"").toUpperCase(),lower:a=>String(a[0]??"").toLowerCase(),trim:a=>String(a[0]??"").trim()
  };
  function eqv(a,b){if(typeof a==="number"||typeof b==="number"){const x=toNum(a),y=toNum(b);if(!isNaN(x)&&!isNaN(y))return x===y;}if(typeof a==="boolean"||typeof b==="boolean")return Boolean(a)===Boolean(b);if(a==null||b==null)return a==null&&b==null;return String(a)===String(b);}
  function ev(n,res){switch(n.type){
    case "lit":return n.v;
    case "ref":return res(n.name);
    case "arr":return n.items.map(x=>ev(x,res));
    case "un":{const v=ev(n.e,res);return n.op==="!"?!v:-toNum(v);}
    case "call":return LIB[n.fn](n.args.map(a=>ev(a,res)));
    case "bin":{const op=n.op;
      if(op==="&&")return !!ev(n.l,res)&&!!ev(n.r,res);
      if(op==="||")return !!ev(n.l,res)||!!ev(n.r,res);
      const a=ev(n.l,res),b=ev(n.r,res);
      switch(op){case "+":return toNum(a)+toNum(b);case "-":return toNum(a)-toNum(b);case "*":return toNum(a)*toNum(b);case "/":return toNum(a)/toNum(b);case "%":return toNum(a)%toNum(b);
        case "<":return toNum(a)<toNum(b);case "<=":return toNum(a)<=toNum(b);case ">":return toNum(a)>toNum(b);case ">=":return toNum(a)>=toNum(b);
        case "==":return eqv(a,b);case "!=":return !eqv(a,b);}}
  }}
  const cache=new Map();
  function parseCached(src){if(cache.has(src))return cache.get(src);const a=parse(src);cache.set(src,a);return a;}
  return {
    parse,
    refsOf(src){const r=[];(function go(n){if(!n)return;if(n.type==="ref")r.push(n.name);else if(n.type==="bin"){go(n.l);go(n.r);}else if(n.type==="un")go(n.e);else if(n.type==="call")n.args.forEach(go);else if(n.type==="arr")n.items.forEach(go);})(parseCached(src));return r;},
    evalSrc(src,res){if(!src)return undefined;let a;try{a=parseCached(src);}catch(_){return undefined;}try{return ev(a,res);}catch(_){return undefined;}}
  };
})();

/* ---- markdown renderer (untuk elemen Keterangan) ---- */
function mdToHtml(src){
  if(!src)return "";
  const e=s=>s.replace(/&/g,"&amp;").replace(/</g,"&lt;").replace(/>/g,"&gt;");
  const inl=s=>e(s).replace(/`([^`]+)`/g,"<code>$1</code>").replace(/\*\*([^*]+)\*\*/g,"<strong>$1</strong>").replace(/(^|[^*])\*([^*]+)\*/g,"$1<em>$2</em>").replace(/\[([^\]]+)\]\(([^)]+)\)/g,'<a href="$2" target="_blank" rel="noopener">$1</a>');
  const L=String(src).replace(/\r\n/g,"\n").split("\n");let h="",i=0;
  while(i<L.length){const l=L[i];
    let m=l.match(/^\s*(#{1,6})\s+(.*)$/);if(m){h+=`<h${m[1].length}>${inl(m[2])}</h${m[1].length}>`;i++;continue;}
    if(/^\s*[-*]\s+/.test(l)){const it=[];while(i<L.length&&/^\s*[-*]\s+/.test(L[i])){it.push(`<li>${inl(L[i].replace(/^\s*[-*]\s+/,""))}</li>`);i++;}h+=`<ul>${it.join("")}</ul>`;continue;}
    if(/^\s*\d+\.\s+/.test(l)){const it=[];while(i<L.length&&/^\s*\d+\.\s+/.test(L[i])){it.push(`<li>${inl(L[i].replace(/^\s*\d+\.\s+/,""))}</li>`);i++;}h+=`<ol>${it.join("")}</ol>`;continue;}
    if(/^\s*>\s?/.test(l)){const q=[];while(i<L.length&&/^\s*>\s?/.test(L[i])){q.push(inl(L[i].replace(/^\s*>\s?/,"")));i++;}h+=`<blockquote>${q.join("<br>")}</blockquote>`;continue;}
    if(/^\s*(-{3,}|\*{3,})\s*$/.test(l)){h+="<hr>";i++;continue;}
    if(/^\s*$/.test(l)){i++;continue;}
    const para=[];while(i<L.length&&!/^\s*$/.test(L[i])&&!/^\s*(#{1,6}\s|[-*]\s|\d+\.\s|>\s?|-{3,}|\*{3,})/.test(L[i])){para.push(inl(L[i]));i++;}
    h+=`<p>${para.join("<br>")}</p>`;
  }
  return h;
}

/* ---- containment rules: parentKind -> allowed childKind ---- */
const ACCEPT={page:["block"],block:["section","roster","field"],section:["field","roster"],roster:["field"]};

let state=blankState(); let selected=null; let view={type:"page",uid:null};
const collapsed={sb1:false,sb2:false};

function blankState(){
  const p={uid:uid(),kind:"page",name:"page_1",title:"Halaman 1",visibleWhen:"",components:[]};
  return {id:"instrumen-baru",title:"Instrumen Baru",version:"1.0.0",acronym:"",locales:["id"],defaultLocale:"id",
    settings:{mode:["capi"],navigation:{mode:"section",showProgress:true,allowBack:true,gateRequired:false}},
    referenceData:{},pages:[p]};
}
function allUsedNames(){const used=new Set();(function walk(arr){(arr||[]).forEach(n=>{if(n.name)used.add(n.name);if(n.components)walk(n.components);});})(state.pages);return used;}
let nameSeq={};
function autoName(t){
  const used=allUsedNames();
  let n=nameSeq[t]||0,name;
  do{n++;name=`${t}_${n}`;}while(used.has(name));
  nameSeq[t]=n;
  return name;
}
function uniqueCopyName(base){const used=allUsedNames();let n=1,name=`${base}_copy`;while(used.has(name)){n++;name=`${base}_copy${n}`;}return name;}
function newPage(){return {uid:uid(),kind:"page",name:autoName("page"),title:"",visibleWhen:"",components:[]};}
function newBlock(){return {uid:uid(),kind:"block",name:autoName("block"),title:"",visibleWhen:"",components:[]};}
function newSection(){return {uid:uid(),kind:"section",name:autoName("section"),title:"",visibleWhen:"",components:[]};}
function newRoster(rt){return {uid:uid(),kind:"roster",name:autoName("roster"),title:"",rowTitle:"",rosterType:rt||"inline",min:"",max:"",countFrom:"",itemLabel:"",rowDisplay:[],visibleWhen:"",components:[]};}
function newField(type){const f={uid:uid(),kind:"field",type,name:autoName(type),label:"",hint:"",required:false,readOnly:false,visibleWhen:"",enableWhen:"",requiredWhen:"",allowRemark:false,defaultValue:""};
  if(CHOICE.has(type)){f.options=[{value:"1",label:"Opsi 1"}];f.optionSource="manual";f.optionsRef="";f.optionsFilterBy="";f.optionsApi={};}
  if(NUMERIC.has(type)){f.min="";f.max="";f.step="";f.unit="";}
  if(TEXTY.has(type)){f.maxLength="";f.pattern="";f.placeholder="";}
  if(type==="calculated")f.calculate=""; if(type==="note")f.html=""; if(type==="markdown")f.markdown="";
  f.skips=[];f.validations=[];return f;}

/* ---- tree helpers ---- */
function* walkAll(arr){for(const n of arr){yield n;if(n.components)yield* walkAll(n.components);}}
function allNodes(){return [...walkAll(state.pages)];}
function findNode(id){for(const n of allNodes())if(n.uid===id)return n;return null;}
function parentArrayOf(id){
  const scan=(arr)=>{for(const n of arr){if(n.uid===id)return arr;if(n.components){const r=scan(n.components);if(r)return r;}}return null;};
  return scan(state.pages);
}
function pageOf(id){
  const inTree=(n,target)=>{if(n.uid===target)return true;return (n.components||[]).some(c=>inTree(c,target));};
  return state.pages.find(p=>inTree(p,id));
}
function removeNode(id){const arr=parentArrayOf(id);if(!arr)return;const i=arr.findIndex(n=>n.uid===id);if(i>=0)arr.splice(i,1);}
function separateRosters(node){const out=[];(function go(n){(n.components||[]).forEach(c=>{if(c.kind==="roster"&&c.rosterType==="separate")out.push(c);go(c);});})(node);return out;}

/* ---- copy & paste ---- */
let clipboard=null; // {kind, data}
function copyNode(node){clipboard={kind:node.kind,data:JSON.parse(JSON.stringify(node))};}
function ownerOf(id){
  function scan(n){for(const c of (n.components||[])){if(c.uid===id)return n;const r=scan(c);if(r)return r;}return null;}
  for(const p of state.pages){if(p.uid===id)return null;const r=scan(p);if(r)return r;}
  return null;
}
function renameDeep(node){if(node.name)node.name=uniqueCopyName(node.name);(node.components||[]).forEach(renameDeep);}
function pasteNode(){
  if(!clipboard)return;
  const copy=JSON.parse(JSON.stringify(clipboard.data));
  reuid(copy);renameDeep(copy);

  if(clipboard.kind==="page"){
    const cur=selected?pageOf(selected):(view.type==="page"?findNode(view.uid):null);
    const idx=cur?state.pages.indexOf(cur):state.pages.length-1;
    state.pages.splice(idx+1,0,copy);
    selected=copy.uid;view={type:"page",uid:copy.uid};render();return;
  }

  const target=selected?findNode(selected):null;
  // 1) coba tempel DI DALAM target bila menerima kind ini
  if(target&&kindAccepted(target.kind,clipboard.kind)){
    target.components=target.components||[];target.components.push(copy);
    selected=copy.uid;render();return;
  }
  // 2) coba tempel sebagai SIBLING setelah target
  if(target){
    const owner=ownerOf(target.uid),ownerKind=owner?owner.kind:"page";
    const arr=parentArrayOf(target.uid);
    if(arr&&kindAccepted(ownerKind,clipboard.kind)){
      const i=arr.indexOf(target);arr.splice(i+1,0,copy);
      selected=copy.uid;render();return;
    }
  }
  // 3) fallback: belum ada target tapi sedang melihat halaman & clipboard berisi block
  if(!target&&view.type==="page"&&clipboard.kind==="block"){
    const pg=findNode(view.uid);
    if(pg){pg.components=pg.components||[];pg.components.push(copy);selected=copy.uid;render();return;}
  }
  alert("Tidak ada lokasi yang cocok untuk elemen ini. Pilih dulu section/block/halaman tujuan, lalu tempel.");
}

/* ===================== PALETTE (sidebar 1) ===================== */
function buildPalette(){
  const pal=document.getElementById("palette");
  let html=`<div class="pal-group"><div class="pal-h">Navigasi</div>
    ${chip("__block","Block (card)","--block")}
    ${chip("__section","Section (border)","--section")}
    ${chip("__roster_inline","Roster — inline","--roster")}
    ${chip("__roster_separate","Roster — subhalaman","--roster")}</div>`;
  const groups=[["Input",TYPES.input],["Pilihan",TYPES.choice],["Tanggal & Waktu",TYPES.time],["Media & Lokasi",TYPES.media],["Lainnya",TYPES.struct]];
  for(const [t,arr] of groups){html+=`<div class="pal-group"><div class="pal-h">${t}</div>`;arr.forEach(x=>html+=chip(x,LABELS[x],CAT_VAR[CAT_OF[x]]));html+=`</div>`;}
  html+=`<div class="hint">Block → Section → field. Roster bisa di Block/Section. Inline tampil di halaman ini; subhalaman muncul di panel Halaman.</div>`;
  pal.innerHTML=html;
  pal.querySelectorAll(".chip").forEach(ch=>ch.addEventListener("dragstart",e=>{dnd.payload={mode:"new",type:ch.dataset.type};e.dataTransfer.effectAllowed="copy";e.dataTransfer.setData("text/plain","new");}));
}
function chip(type,label,v){const ty=type.startsWith("__")?type.replace("__","").replace("_"," "):type;return `<div class="chip" draggable="true" data-type="${type}" style="--cat:var(${v})"><span class="rail2"></span><span class="nm">${label}</span><span class="ty">${ty}</span></div>`;}
function newFromType(t){return t==="__block"?newBlock():t==="__section"?newSection():t==="__roster_inline"?newRoster("inline"):t==="__roster_separate"?newRoster("separate"):newField(t);}
function kindAccepted(parentKind,childKind){return (ACCEPT[parentKind]||[]).includes(childKind);}

/* ===================== PAGE NAVIGATOR (sidebar 2) ===================== */
function renderPages(){
  const list=document.getElementById("pageList"); list.innerHTML="";
  state.pages.forEach((p,i)=>{
    const row=document.createElement("div");
    row.className="pg"+(view.type==="page"&&view.uid===p.uid?" active":""); row.draggable=true; row.dataset.uid=p.uid;
    row.innerHTML=`<span class="pt">${esc(p.title||p.name)}</span><button class="px" title="Hapus halaman">×</button>`;
    row.addEventListener("click",e=>{if(e.target.classList.contains("px"))return;openPage(p.uid);select(p.uid);});
    row.querySelector(".px").addEventListener("click",ev=>{ev.stopPropagation();if(state.pages.length<=1){alert("Minimal satu halaman.");return;}if(confirm("Hapus halaman ini?")){removeNode(p.uid);view={type:"page",uid:state.pages[0].uid};selected=null;render();}});
    row.addEventListener("dragstart",e=>{e.stopPropagation();dnd.payload={mode:"page",id:p.uid};row.classList.add("dragging");});
    row.addEventListener("dragend",()=>row.classList.remove("dragging"));
    row.addEventListener("dragover",e=>{if(dnd.payload&&dnd.payload.mode==="page"){e.preventDefault();}});
    row.addEventListener("drop",e=>{if(dnd.payload&&dnd.payload.mode==="page"){e.preventDefault();reorderPage(dnd.payload.id,p.uid);dnd.payload=null;render();}});
    list.appendChild(row);
    separateRosters(p).forEach(r=>{
      const sp=document.createElement("div");
      sp.className="subpg"+(view.type==="roster"&&view.uid===r.uid?" active":"");
      sp.innerHTML=`<span class="ri">⊞</span><span>${esc(r.title||r.name)}</span>`;
      sp.addEventListener("click",()=>{view={type:"roster",uid:r.uid};select(r.uid);render();});
      list.appendChild(sp);
    });
  });
}
function reorderPage(dragId,targetId){const a=state.pages;const from=a.findIndex(p=>p.uid===dragId),to=a.findIndex(p=>p.uid===targetId);if(from<0||to<0)return;const [m]=a.splice(from,1);a.splice(to,0,m);}
function openPage(id){view={type:"page",uid:id};}

/* ===================== CANVAS ===================== */
function render(){
  if(!state.pages.find(p=>p.uid===view.uid) && view.type==="page") view={type:"page",uid:state.pages[0].uid};
  if(view.type==="roster" && !findNode(view.uid)) view={type:"page",uid:state.pages[0].uid};
  document.getElementById("instTitle").value=state.title||"";
  renderPages(); renderCanvas(); renderInspector(); runValidation(); applyCols();
}
function refreshCard(node){const el=document.querySelector(`#stage [data-uid="${node.uid}"]`);if(el)el.replaceWith(renderNode(node));}
function softUpdate(){renderPages();runValidation();const n=selected&&findNode(selected);if(n)refreshCard(n);}
function renderCanvas(){
  const head=document.getElementById("cvHead"), stage=document.getElementById("stage"); stage.innerHTML="";
  if(view.type==="roster"){
    const r=findNode(view.uid), pg=pageOf(r.uid);
    head.innerHTML=`<div class="eyebrow">Template baris roster</div><button class="btn ghost back" id="backBtn">← ${esc(pg.title||pg.name)}</button>`;
    head.querySelector("#backBtn").addEventListener("click",()=>{openPage(pg.uid);select(r.uid);render();});
    const wrap=document.createElement("div");wrap.className="roster-inline";
    wrap.appendChild(nodeHead(r,"roster",`Roster: ${r.title||r.name}`));
    wrap.appendChild(dropzone(r,["field"],"Seret field — ini diulang tiap baris"));
    stage.appendChild(wrap); return;
  }
  const page=findNode(view.uid);
  head.innerHTML=`<div class="eyebrow">Halaman · ${esc(page.title||page.name)}</div>`;
  stage.appendChild(dropzone(page,["block"],"Seret Block ke halaman ini"));
}

function nodeHead(node,kind,placeholder){
  const h=document.createElement("div"); h.className="node-head";
  h.innerHTML=`<span class="tag ${kind}">${kind}</span><input class="ti" value="${esc(node.title||"")}" placeholder="${esc(placeholder||"judul "+kind+" (opsional)")}">
    <button class="icon-btn" data-a="sel" title="Atur">⚙</button><button class="icon-btn danger" data-a="del" title="Hapus">🗑</button>`;
  h.querySelector(".ti").addEventListener("input",e=>{node.title=e.target.value;runValidation();renderPages();});
  h.querySelector('[data-a="sel"]').addEventListener("click",e=>{e.stopPropagation();select(node.uid);});
  h.querySelector('[data-a="del"]').addEventListener("click",e=>{e.stopPropagation();if(confirm(`Hapus ${kind} ini beserta isinya?`)){removeNode(node.uid);selected=null;render();}});
  h.addEventListener("click",e=>{if(!e.target.closest("input,button"))select(node.uid);});
  return h;
}

function dropzone(owner,accept,emptyText){
  const arr=owner.components;
  const dz=document.createElement("div"); dz.className="dropzone"+(arr.length===0?" empty":"");
  dz.dataset.owner=owner.uid;
  if(arr.length===0) dz.textContent=emptyText;
  else arr.forEach(n=>dz.appendChild(renderNode(n)));
  wireDropzone(dz,owner,accept);
  return dz;
}

function renderNode(n){
  if(n.kind==="block"){
    const el=document.createElement("div");el.className="block"+(selected===n.uid?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"block"));
    el.appendChild(dropzone(n,["section","roster","field"],"Seret Section, Roster, atau field ke dalam block"));
    wireDrag(el,n); return el;
  }
  if(n.kind==="section"){
    const el=document.createElement("div");el.className="section"+(selected===n.uid?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"section"));
    el.appendChild(dropzone(n,["field","roster"],"Seret field ke dalam section"));
    wireDrag(el,n); return el;
  }
  if(n.kind==="roster"){
    if(n.rosterType==="separate"){
      const el=document.createElement("div");el.className="roster-link"+(selected===n.uid?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
      el.innerHTML=`<span class="ri">⊞</span><div class="rt"><b>${esc(n.title||n.name)}</b><span>Roster subhalaman · ${(n.components||[]).length} field${n.countFrom?" · ×"+esc(n.countFrom):""}</span></div><span class="go">buka →</span>`;
      el.addEventListener("click",e=>{if(e.target.closest(".go")||!e.target.closest("button")){ if(e.detail===2){view={type:"roster",uid:n.uid};} select(n.uid);} render();});
      el.querySelector(".go").addEventListener("click",e=>{e.stopPropagation();view={type:"roster",uid:n.uid};select(n.uid);render();});
      wireDrag(el,n); return el;
    }
    const el=document.createElement("div");el.className="roster-inline"+(selected===n.uid?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"roster",`Roster inline: ${n.title||n.name}`));
    el.appendChild(dropzone(n,["field"],"Seret field — diulang tiap baris"));
    wireDrag(el,n); return el;
  }
  // field card
  const cat=CAT_OF[n.type];
  const el=document.createElement("div");el.className="card"+(selected===n.uid?" sel":"");el.dataset.uid=n.uid;el.draggable=true;el.style.setProperty("--cat",`var(${CAT_VAR[cat]})`);
  const badges=[`<span class="badge nm">${esc(n.name||"?")}</span>`];
  if(n.skips&&n.skips.length)badges.push(`<span class="badge skip">skip →</span>`);
  if(n.optionSource==="api"||(n.optionsApi&&n.optionsApi.url))badges.push(`<span class="badge">API</span>`);
  else if(n.optionsRef)badges.push(`<span class="badge">ref:${esc(n.optionsRef)}</span>`);
  else if(n.options&&n.options.length)badges.push(`<span class="badge">${n.options.length} opsi</span>`);
  if(n.validations&&n.validations.length)badges.push(`<span class="badge">${n.validations.length} cek</span>`);
  if(n.visibleWhen)badges.push(`<span class="badge">⊘ kondisi</span>`);
  const lbl=(n.type==="note"||n.type==="markdown")?`<span style="color:var(--ink-soft)">${esc(((n.type==="markdown"?(n.markdown||""):String(n.html||"").replace(/<[^>]+>/g," ")).replace(/[#>*`_-]/g," ").trim().slice(0,70))||"(keterangan kosong)")}</span>`:(n.label?esc(n.label):`<span class="empty">tanpa label</span>`);
  el.innerHTML=`<div class="crail"></div><div class="body"><div class="top"><span class="ty">${n.type}</span>${n.required?'<span class="req">＊</span>':''}</div><div class="lbl">${lbl}</div><div class="meta">${badges.join("")}</div></div><div class="grip">⋮⋮</div>`;
  el.querySelector(".body").addEventListener("click",()=>select(n.uid));
  wireDrag(el,n); return el;
}

/* ===================== DRAG & DROP ===================== */
const dnd={payload:null}; let placeholder=null;
function clearPlaceholder(){if(placeholder&&placeholder.parentNode)placeholder.parentNode.removeChild(placeholder);placeholder=null;}
function wireDrag(el,n){
  el.addEventListener("dragstart",e=>{e.stopPropagation();dnd.payload={mode:"move",id:n.uid};el.classList.add("dragging");e.dataTransfer.effectAllowed="move";e.dataTransfer.setData("text/plain","move");});
  el.addEventListener("dragend",()=>{el.classList.remove("dragging");clearPlaceholder();});
}
function draggedKind(){if(!dnd.payload)return null;if(dnd.payload.mode==="new"){const t=dnd.payload.type;return t==="__block"?"block":t==="__section"?"section":t.startsWith("__roster")?"roster":"field";}const node=findNode(dnd.payload.id);return node?node.kind:null;}
function wireDropzone(dz,owner,accept){
  dz.addEventListener("dragover",e=>{
    if(!dnd.payload||dnd.payload.mode==="page")return;
    e.preventDefault();e.stopPropagation();
    const k=draggedKind();
    if(!accept.includes(k)){dz.classList.add("reject");dz.classList.remove("over");clearPlaceholder();return;}
    if(dnd.payload.mode==="move"){const dragged=findNode(dnd.payload.id);if(dragged&&nodeContains(dragged,owner.uid))return;}
    dz.classList.remove("reject");dz.classList.add("over");
    if(!placeholder){placeholder=document.createElement("div");placeholder.className="placeholder";}
    const after=childAfter(dz,e.clientY); if(after==null)dz.appendChild(placeholder);else dz.insertBefore(placeholder,after);
  });
  dz.addEventListener("dragleave",e=>{if(e.target===dz){dz.classList.remove("over");dz.classList.remove("reject");}});
  dz.addEventListener("drop",e=>{
    if(!dnd.payload||dnd.payload.mode==="page")return;
    e.preventDefault();e.stopPropagation();
    dz.classList.remove("over");dz.classList.remove("reject");
    const k=draggedKind();
    if(!accept.includes(k)){clearPlaceholder();dnd.payload=null;return;}
    const arr=owner.components; const index=phIndex(dz,arr); clearPlaceholder();
    if(dnd.payload.mode==="new"){const node=newFromType(dnd.payload.type);arr.splice(index,0,node);selected=node.uid;}
    else{const id=dnd.payload.id,dragged=findNode(id),src=parentArrayOf(id);
      if(dragged&&src&&!nodeContains(dragged,owner.uid)){const from=src.indexOf(dragged);src.splice(from,1);let ins=index;if(src===arr&&from<index)ins--;arr.splice(ins,0,dragged);}}
    dnd.payload=null;render();
  });
}
function childAfter(dz,y){const items=[...dz.children].filter(c=>c!==placeholder&&(c.classList.contains("card")||c.classList.contains("block")||c.classList.contains("section")||c.classList.contains("roster-inline")||c.classList.contains("roster-link")));for(const c of items){const r=c.getBoundingClientRect();if(y<r.top+r.height/2)return c;}return null;}
function phIndex(dz,arr){const kids=[...dz.children].filter(c=>c===placeholder||c.dataset&&c.dataset.uid);const i=kids.indexOf(placeholder);return i<0?arr.length:i;}
function nodeContains(node,targetUid){if(node.uid===targetUid)return true;return (node.components||[]).some(c=>nodeContains(c,targetUid));}

/* ===================== SELECTION & INSPECTOR ===================== */
function select(id){selected=id;const n=findNode(id);if(n&&view.type==="page"){const pg=pageOf(id);if(pg&&pg.uid!==view.uid)view={type:"page",uid:pg.uid};}switchTab("props");render();}
function renderInspector(){
  const pane=document.getElementById("paneProps");const n=selected?findNode(selected):null;
  if(!n){pane.innerHTML=instrumentForm();wireInstrument(pane);return;}
  if(n.kind==="page")pane.innerHTML=navForm(n,"page");
  else if(n.kind==="block")pane.innerHTML=navForm(n,"block");
  else if(n.kind==="section")pane.innerHTML=navForm(n,"section");
  else if(n.kind==="roster")pane.innerHTML=rosterForm(n);
  else pane.innerHTML=fieldForm(n);
  wireForm(pane,n);
}
function instrumentForm(){const nv=state.settings.navigation;return `<div class="empty-state"><div class="big">{ }</div>Tidak ada yang dipilih — pengaturan instrumen.</div>
  <div class="field"><label>ID instrumen</label><input class="ctrl mono" data-i="id" value="${esc(state.id)}"></div>
  <div class="row2"><div class="field"><label>Versi</label><input class="ctrl mono" data-i="version" value="${esc(state.version)}"></div><div class="field"><label>Akronim</label><input class="ctrl" data-i="acronym" value="${esc(state.acronym||"")}"></div></div>
  <div class="row2"><div class="field"><label>Locales</label><input class="ctrl mono" data-i="locales" value="${esc(state.locales.join(","))}"></div><div class="field"><label>Locale utama</label><input class="ctrl mono" data-i="defaultLocale" value="${esc(state.defaultLocale)}"></div></div>
  <div class="group"><div class="gh">Navigasi</div>
    <div class="field"><label>Mode</label><select class="ctrl" data-i="nav.mode">${opt("scroll","Scroll",nv.mode)}${opt("section","Section per halaman",nv.mode)}${opt("field","Field per layar",nv.mode)}</select></div>
    <label class="check"><input type="checkbox" data-i="nav.gateRequired" ${nv.gateRequired?"checked":""}> Wajib selesai sebelum lanjut</label></div>
  <div class="group"><div class="gh">Sumber lookup / Reference data (JSON)</div><textarea class="ctrl mono" data-i="referenceData" rows="6" placeholder='{ "kabupaten": { "items":[ {"code":"6472","label":"Samarinda"} ] } }'>${esc(jsonOrEmpty(state.referenceData))}</textarea><div class="help" style="margin-left:0;margin-top:6px">Tiap tabel bisa <b>inline</b> (pakai <code>items</code>) atau <b>API</b>:<br><code>"kec": { "source":"api", "url":"https://.../kec?prov={parent}", "valueField":"kode", "labelField":"nama", "parentParam":"prov", "path":"data" }</code><br>API: <code>valueField</code>/<code>labelField</code>=key di respons; <code>{parent}</code> atau <code>parentParam</code> untuk cascading; <code>path</code> bila array bersarang. Rujuk dari field lewat <b>optionsRef</b>.</div></div>`;}
function wireInstrument(pane){pane.querySelectorAll("[data-i]").forEach(inp=>inp.addEventListener("input",()=>{const k=inp.dataset.i,v=inp.type==="checkbox"?inp.checked:inp.value;if(k.startsWith("nav."))state.settings.navigation[k.slice(4)]=v;else if(k==="locales")state.locales=v.split(",").map(s=>s.trim()).filter(Boolean);else if(k==="referenceData"){try{state.referenceData=v.trim()?JSON.parse(v):{};inp.style.borderColor="";}catch(_){inp.style.borderColor="var(--bad)";}}else state[k]=v;runValidation();}));}

function navForm(n,kind){
  const titleLabel=kind==="page"?"Judul halaman":kind==="block"?"Judul block (opsional)":"Judul section (opsional)";
  // Halaman & section: dataKey dibuat otomatis & sudah pasti unik, tapi tetap bisa diubah bila perlu.
  const nameField=(kind==="page"||kind==="section")
    ? `<div class="field"><label>Nama (dataKey) <span class="help">otomatis & unik, bisa diubah</span></label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>`
    : `<div class="field"><label>Nama (dataKey) <span class="help">unik</span></label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>`;
  return `${headBar(kind,n.name)}
  ${nameField}
  <div class="field"><label>${titleLabel}</label><input class="ctrl" data-k="title" value="${esc(n.title||"")}"></div>
  <div class="field"><label>Tampil bila (visibleWhen)</label><textarea class="ctrl" data-k="visibleWhen" placeholder="\${field} == nilai">${esc(n.visibleWhen||"")}</textarea></div>`;
}
function rosterForm(n){
  const childFields=(n.components||[]).filter(c=>c.kind==="field");
  const dispBlock = `<div class="group"><div class="gh">Field tampil di daftar baris</div>${childFields.length?childFields.map(f=>`<label class="check"><input type="checkbox" data-rowdisp="${esc(f.name)}" ${(n.rowDisplay||[]).includes(f.name)?"checked":""}> ${esc(f.label||f.name)}</label>`).join(""):`<div class="help" style="margin-left:0">Tambah field ke roster dulu.</div>`}<div class="help" style="margin-left:0;margin-top:6px">Untuk roster subhalaman: nilai field ini jadi ringkasan tiap baris di halaman utama.</div></div>`;
  return `${headBar("roster",n.name)}
  <div class="field"><label>Jenis roster</label>
    <div class="seg" id="rtSeg"><button data-rt="inline" class="${n.rosterType==="inline"?"on":""}">Inline</button><button data-rt="separate" class="${n.rosterType==="separate"?"on":""}">Subhalaman</button></div>
    <div class="help" style="margin-left:0;margin-top:6px">${n.rosterType==="inline"?"Input di halaman yang sama.":"Daftar baris di halaman utama; isi tiap baris di halaman terpisah."}</div></div>
  <div class="field"><label>Judul roster (opsional)</label><input class="ctrl" data-k="title" value="${esc(n.title||"")}"></div>
  <div class="field"><label>Judul baris roster <span class="help">mis. "Usaha" — dipakai di tombol &amp; popup tambah baris</span></label><input class="ctrl" data-k="rowTitle" placeholder="Usaha" value="${esc(n.rowTitle||"")}"></div>
  <div class="field"><label>Nama (dataKey)</label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>
  <div class="row2"><div class="field"><label>Min baris</label><input class="ctrl" type="number" step="1" min="0" inputmode="numeric" data-k="min" value="${esc(n.min??"")}"></div><div class="field"><label>Maks baris</label><input class="ctrl" type="number" step="1" min="0" inputmode="numeric" data-k="max" value="${esc(n.max??"")}"></div></div>
  <div class="field"><label>Jumlah baris dari field (countFrom) <span class="help">otomatis generate baris; kosongkan untuk pakai tombol "+ Tambah ${n.rowTitle?esc(n.rowTitle):"baris"}" dengan popup</span></label><input class="ctrl mono" data-k="countFrom" value="${esc(n.countFrom||"")}"></div>
  <div class="field"><label>Label tiap baris (itemLabel)</label><input class="ctrl" data-k="itemLabel" placeholder="Usaha {{index}}: \${nama}" value="${esc(n.itemLabel||"")}"></div>
  ${dispBlock}
  <div class="field"><label>Tampil bila</label><textarea class="ctrl" data-k="visibleWhen">${esc(n.visibleWhen||"")}</textarea></div>
  ${n.rosterType==="separate"?`<button class="add-row" id="openRoster">Buka editor template roster →</button>`:""}`;
}
function fieldForm(c){const t=c.type;let html=headBar(t,c.name);
  html+=`<div class="field"><label>Nama (dataKey) <span class="help">unik, kolom output</span></label><input class="ctrl mono" data-k="name" value="${esc(c.name)}"></div>`;
  if(t!=="note")html+=`<div class="field"><label>Label pertanyaan</label><input class="ctrl" data-k="label" value="${esc(c.label||"")}"></div>`;
  html+=`<div class="field"><label>Petunjuk (hint)</label><input class="ctrl" data-k="hint" value="${esc(c.hint||"")}"></div>`;
  if(t==="note")html+=`<div class="field"><label>Konten HTML</label><textarea class="ctrl" data-k="html" rows="3">${esc(c.html||"")}</textarea></div>`;
  if(t==="markdown")html+=`<div class="field"><label>Keterangan (Markdown)</label><textarea class="ctrl" data-k="markdown" rows="6" placeholder="# Petunjuk Pengisian&#10;&#10;Isi sesuai **kondisi sebenarnya**. Lihat:&#10;- poin pertama&#10;- poin kedua&#10;&#10;> Catatan penting.">${esc(c.markdown||"")}</textarea><div class="help" style="margin-left:0;margin-top:4px">Mendukung: # judul, **tebal**, *miring*, \`kode\`, list (- / 1.), &gt; kutipan, [teks](url), --- garis.</div></div>`;
  if(t==="calculated")html+=`<div class="field"><label>Rumus (calculate)</label><textarea class="ctrl" data-k="calculate" placeholder="\${a}+\${b}">${esc(c.calculate||"")}</textarea></div>`;
  if(NUMERIC.has(t))html+=`<div class="row3">${mini("min","Min",c.min)}${mini("max","Maks",c.max)}${mini("step","Step",c.step)}</div><div class="field"><label>Satuan</label><input class="ctrl" data-k="unit" value="${esc(c.unit||"")}"></div>`;
  if(TEXTY.has(t))html+=`<div class="row2">${mini("maxLength","Maks karakter",c.maxLength,"number")}<div class="field"><label>Placeholder</label><input class="ctrl" data-k="placeholder" value="${esc(c.placeholder||"")}"></div></div><div class="field"><label>Pola (regex)</label><input class="ctrl mono" data-k="pattern" value="${esc(c.pattern||"")}"></div>`;
  if(CHOICE.has(t))html+=optionsBlock(c);
  html+=`<div class="group"><div class="gh">Perilaku</div><label class="check"><input type="checkbox" data-k="required" ${c.required?"checked":""}> Wajib diisi</label><label class="check"><input type="checkbox" data-k="readOnly" ${c.readOnly?"checked":""}> Hanya baca</label><label class="check"><input type="checkbox" data-k="allowRemark" ${c.allowRemark?"checked":""}> Izinkan catatan</label></div>`;
  html+=`<div class="group"><div class="gh">Kondisi & alur</div>${cond("visibleWhen","Tampil bila",c.visibleWhen)}${cond("enableWhen","Aktif bila",c.enableWhen)}${cond("requiredWhen","Wajib bila",c.requiredWhen)}${skipsBlock(c)}</div>`;
  html+=validationsBlock(c); return html;
}
function headBar(kind,name){const cat=CAT_OF[kind]||"node";const colorVar=({page:"--page",block:"--block",section:"--section",roster:"--roster"})[kind]||CAT_VAR[cat];const pasteBtn=clipboard?`<button class="icon-btn" id="pasteBtn" title="Tempel ${esc(clipboard.kind)} yang disalin">📥</button>`:"";return `<div style="display:flex;align-items:center;gap:9px;margin-bottom:14px"><span style="width:4px;height:30px;border-radius:2px;background:var(${colorVar})"></span><div><div style="font-family:var(--mono);font-size:11px;color:var(${colorVar});font-weight:700;text-transform:uppercase">${kind}</div><div style="font-size:11px;color:var(--muted)">${esc(name)}</div></div><button class="icon-btn" id="copyBtn" title="Salin" style="margin-left:auto">📋</button><button class="icon-btn" id="dupBtn" title="Duplikat">⧉</button>${pasteBtn}<button class="icon-btn danger" id="delBtn" title="Hapus">🗑</button></div>`;}
function mini(k,l,v,type){return `<div class="field"><label>${l}</label><input class="ctrl" ${type==="number"?'type="number" step="1" min="0" inputmode="numeric"':''} data-k="${k}" value="${esc(v??"")}"></div>`;}
function cond(k,l,v){return `<div class="field"><label>${l}</label><textarea class="ctrl" data-k="${k}" placeholder="\${field} == nilai">${esc(v||"")}</textarea></div>`;}
function optionsBlock(c){
  const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
  const seg=`<div class="seg" id="osSeg"><button data-os="manual" class="${mode==="manual"?"on":""}">Manual</button><button data-os="ref" class="${mode==="ref"?"on":""}">Inline</button><button data-os="api" class="${mode==="api"?"on":""}">API</button></div>`;
  let body="";
  if(mode==="manual"){
    const rows=(c.options||[]).map((o,i)=>`<div class="mini" data-oi="${i}"><div class="mr"><input class="ctrl" data-of="value" placeholder="value" value="${esc(o.value??"")}"><input class="ctrl" data-of="label" placeholder="label" value="${esc(typeof o.label==="object"?(o.label.id||""):(o.label||""))}"><button class="x" data-orm>×</button></div><input class="ctrl mono" data-of="skipTo" placeholder="skipTo (opsional)" value="${esc(o.skipTo||"")}" style="margin-top:6px"></div>`).join("");
    body=`<div id="optRows">${rows}</div><button class="add-row" id="addOpt">+ Tambah opsi</button>`;
  } else if(mode==="ref"){
    const tables=Object.entries(state.referenceData||{}).filter(([k,v])=>!(v&&v.source==="api")).map(([k])=>k);
    body = tables.length
      ? `<div class="field"><label>Tabel sumber (variabel)</label><select class="ctrl" data-k="optionsRef"><option value="">— pilih tabel —</option>${tables.map(k=>`<option value="${esc(k)}"${c.optionsRef===k?" selected":""}>${esc(k)}</option>`).join("")}</select></div>`
      : `<div class="help" style="margin-left:0">Belum ada tabel inline. Definisikan dulu di pengaturan instrumen → Reference data, lalu pilih di sini.</div>`;
    body+=`<div class="field"><label>Filter berjenjang (field induk)</label><input class="ctrl mono" data-k="optionsFilterBy" placeholder="nama field induk (opsional)" value="${esc(c.optionsFilterBy||"")}"></div>`;
  } else {
    const a=c.optionsApi||{};
    body=`<div class="field"><label>URL API <span class="help">gunakan {dataKey} untuk substitusi nilai field</span></label><input class="ctrl mono" data-api="url" placeholder="https://api.../wilayah?kab={kabupaten_kota}" value="${esc(a.url||"")}"></div>
      <div class="field"><label>Trigger dataKey <span class="help">dataKey yang memicu fetch ulang &amp; harus terisi dulu — pisah koma</span></label><input class="ctrl mono" data-api="depKeys" placeholder="provinsi, kabupaten_kota" value="${esc(a.depKeys||"")}"></div>
      <div class="row2"><div class="field"><label>Value field</label><input class="ctrl mono" data-api="valueField" placeholder="kode" value="${esc(a.valueField||"")}"></div><div class="field"><label>Label field</label><input class="ctrl mono" data-api="labelField" placeholder="nama" value="${esc(a.labelField||"")}"></div></div>
      <div class="row2"><div class="field"><label>Parent param <span class="help">cascading</span></label><input class="ctrl mono" data-api="parentParam" placeholder="prov (opsional)" value="${esc(a.parentParam||"")}"></div><div class="field"><label>Path respons <span class="help">opsional</span></label><input class="ctrl mono" data-api="path" placeholder="data" value="${esc(a.path||"")}"></div></div>
      <div class="field"><label>Filter berjenjang (field induk)</label><input class="ctrl mono" data-k="optionsFilterBy" placeholder="nama field induk (opsional)" value="${esc(c.optionsFilterBy||"")}"></div>
      <div class="help" style="margin-left:0"><code>{dataKey}</code> di URL diganti nilai field tersebut. Trigger dataKey memblokir fetch &amp; mereset pilihan saat belum terisi. <code>path</code> bila array bersarang.</div>`;
  }
  return `<div class="group"><div class="gh">Pilihan · sumber</div>${seg}${body}</div>`;
}
function skipsBlock(c){let rows=(c.skips||[]).map((s,i)=>`<div class="mini" data-si="${i}"><input class="ctrl mono" data-sf="when" placeholder="bila (ekspresi)" value="${esc(s.when||"")}"><div class="mr" style="grid-template-columns:1fr auto;margin-top:6px"><input class="ctrl mono" data-sf="to" placeholder="lompat ke / __end" value="${esc(s.to||"")}"><button class="x" data-srm>×</button></div></div>`).join("");return `<div style="margin-top:6px"><div class="gh" style="margin-bottom:6px">Lompatan (skips)</div><div id="skipRows">${rows}</div><button class="add-row" id="addSkip">+ Tambah lompatan</button></div>`;}
function validationsBlock(c){let rows=(c.validations||[]).map((v,i)=>`<div class="mini" data-vi="${i}"><input class="ctrl mono" data-vf="test" placeholder="test (TRUE=lolos)" value="${esc(v.test||"")}"><div class="mr" style="grid-template-columns:1fr auto;margin-top:6px"><input class="ctrl" data-vf="message" placeholder="pesan" value="${esc(typeof v.message==="object"?(v.message.id||""):(v.message||""))}"><button class="x" data-vrm>×</button></div><select class="ctrl" data-vf="severity" style="margin-top:6px">${opt("error","error — blokir",v.severity||"error")}${opt("warning","warning — boleh lanjut",v.severity||"error")}</select></div>`).join("");return `<div class="group"><div class="gh">Validasi</div><div id="valRows">${rows}</div><button class="add-row" id="addVal">+ Tambah aturan</button></div>`;}

function wireForm(pane,node){
  pane.querySelectorAll("[data-k]").forEach(inp=>{const h=()=>{node[inp.dataset.k]=inp.type==="checkbox"?inp.checked:inp.value;softUpdate();};inp.addEventListener("input",h);inp.addEventListener("change",h);});
  pane.querySelector("#delBtn")?.addEventListener("click",()=>{if(confirm("Hapus ini?")){removeNode(node.uid);selected=null;render();}});
  pane.querySelector("#dupBtn")?.addEventListener("click",()=>{const arr=parentArrayOf(node.uid),i=arr.indexOf(node),copy=JSON.parse(JSON.stringify(node));reuid(copy);copy.name=uniqueCopyName(node.name);arr.splice(i+1,0,copy);selected=copy.uid;render();});
  pane.querySelector("#copyBtn")?.addEventListener("click",()=>{copyNode(node);render();});
  pane.querySelector("#pasteBtn")?.addEventListener("click",()=>{pasteNode();});
  pane.querySelectorAll("#rtSeg button").forEach(b=>b.addEventListener("click",()=>{node.rosterType=b.dataset.rt;render();}));
  pane.querySelectorAll("[data-rowdisp]").forEach(cb=>cb.addEventListener("change",()=>{node.rowDisplay=node.rowDisplay||[];const nm=cb.getAttribute("data-rowdisp");if(cb.checked){if(!node.rowDisplay.includes(nm))node.rowDisplay.push(nm);}else node.rowDisplay=node.rowDisplay.filter(x=>x!==nm);softUpdate();}));
  pane.querySelector("#openRoster")?.addEventListener("click",()=>{view={type:"roster",uid:node.uid};render();});
  // options
  pane.querySelectorAll("#osSeg button").forEach(b=>b.addEventListener("click",()=>{node.optionSource=b.dataset.os;if(node.optionSource==="api"&&!node.optionsApi)node.optionsApi={url:"",valueField:"",labelField:""};render();}));
  pane.querySelectorAll("[data-api]").forEach(inp=>inp.addEventListener("input",()=>{node.optionsApi=node.optionsApi||{};node.optionsApi[inp.dataset.api]=inp.value;softUpdate();}));
  pane.querySelectorAll("#optRows .mini").forEach(row=>{const i=+row.dataset.oi;row.querySelectorAll("[data-of]").forEach(inp=>inp.addEventListener("input",()=>{const f=inp.dataset.of;node.options[i][f]=inp.value;softUpdate();}));row.querySelector("[data-orm]")?.addEventListener("click",()=>{node.options.splice(i,1);render();});});
  pane.querySelector("#addOpt")?.addEventListener("click",()=>{node.options=node.options||[];node.options.push({value:String(node.options.length+1),label:"Opsi "+(node.options.length+1)});render();});
  // skips
  pane.querySelectorAll("#skipRows .mini").forEach(row=>{const i=+row.dataset.si;row.querySelectorAll("[data-sf]").forEach(inp=>inp.addEventListener("input",()=>{node.skips[i][inp.dataset.sf]=inp.value;softUpdate();}));row.querySelector("[data-srm]")?.addEventListener("click",()=>{node.skips.splice(i,1);render();});});
  pane.querySelector("#addSkip")?.addEventListener("click",()=>{node.skips=node.skips||[];node.skips.push({when:"",to:""});render();});
  // validations
  pane.querySelectorAll("#valRows .mini").forEach(row=>{const i=+row.dataset.vi;row.querySelectorAll("[data-vf]").forEach(inp=>inp.addEventListener("input",()=>{node.validations[i][inp.dataset.vf]=inp.value;softUpdate();}));row.querySelector("[data-vrm]")?.addEventListener("click",()=>{node.validations.splice(i,1);render();});});
  pane.querySelector("#addVal")?.addEventListener("click",()=>{node.validations=node.validations||[];node.validations.push({test:"",message:"",severity:"error"});render();});
}

/* ===================== SERIALIZE ===================== */
function clean(v){return v!==""&&v!=null;}
function loc(t){return {[state.defaultLocale]:t};}
function serialize(){const out={specVersion:"1.1",id:state.id,title:loc(state.title),version:state.version};if(state.acronym)out.acronym=state.acronym;out.locales=state.locales;out.defaultLocale=state.defaultLocale;out.settings=JSON.parse(JSON.stringify(state.settings));if(Object.keys(state.referenceData||{}).length)out.referenceData=state.referenceData;out.pages=state.pages.map(serNode);return out;}
function serNode(n){
  if(n.kind==="page"){const o={kind:"page",name:n.name};if(clean(n.title))o.title=loc(n.title);if(clean(n.visibleWhen))o.visibleWhen=n.visibleWhen;o.components=n.components.map(serNode);return o;}
  if(n.kind==="block"){const o={kind:"block",name:n.name,layout:"card"};if(clean(n.title))o.title=loc(n.title);if(clean(n.visibleWhen))o.visibleWhen=n.visibleWhen;o.components=n.components.map(serNode);return o;}
  if(n.kind==="section"){const o={kind:"section",name:n.name,layout:"bordered"};if(clean(n.title))o.title=loc(n.title);if(clean(n.visibleWhen))o.visibleWhen=n.visibleWhen;o.components=n.components.map(serNode);return o;}
  if(n.kind==="roster"){const o={kind:"roster",name:n.name,rosterType:n.rosterType};if(clean(n.title))o.title=loc(n.title);if(clean(n.rowTitle))o.rowTitle=n.rowTitle;["min","max"].forEach(k=>{if(clean(n[k]))o[k]=num(n[k]);});if(clean(n.countFrom))o.countFrom=n.countFrom;if(clean(n.itemLabel))o.itemLabel=loc(n.itemLabel);if(n.rowDisplay&&n.rowDisplay.length)o.rowDisplay=n.rowDisplay;if(clean(n.visibleWhen))o.visibleWhen=n.visibleWhen;o.components=n.components.map(serNode);return o;}
  const c=n,o={kind:"field",name:c.name,type:c.type};
  if(c.type!=="note"&&clean(c.label))o.label=loc(c.label);
  if(clean(c.hint))o.hint=loc(c.hint);
  if(c.type==="note"&&clean(c.html))o.html=loc(c.html);
  if(c.type==="markdown"&&clean(c.markdown))o.markdown=loc(c.markdown);
  if(c.type==="calculated"&&clean(c.calculate))o.calculate=c.calculate;
  if(clean(c.unit))o.unit=c.unit;
  ["min","max","step","maxLength"].forEach(k=>{if(clean(c[k]))o[k]=num(c[k]);});
  if(clean(c.pattern))o.pattern=c.pattern;if(clean(c.placeholder))o.placeholder=loc(c.placeholder);
  if(CHOICE.has(c.type)){const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
    if(mode==="api"&&c.optionsApi&&c.optionsApi.url){const a={url:c.optionsApi.url};["valueField","labelField","parentParam","searchParam","path","method","depKeys"].forEach(k=>{if(clean(c.optionsApi[k]))a[k]=c.optionsApi[k];});o.optionsApi=a;if(clean(c.optionsFilterBy))o.optionsFilterBy=c.optionsFilterBy;}
    else if(mode==="ref"&&clean(c.optionsRef)){o.optionsRef=c.optionsRef;if(clean(c.optionsFilterBy))o.optionsFilterBy=c.optionsFilterBy;}
    else if(c.options&&c.options.length)o.options=c.options.map(op=>{const x={value:coerce(op.value)};if(clean(op.label))x.label=loc(op.label);if(clean(op.skipTo))x.skipTo=op.skipTo;return x;});}
  ["required","readOnly","allowRemark"].forEach(k=>{if(c[k])o[k]=true;});
  ["visibleWhen","enableWhen","requiredWhen"].forEach(k=>{if(clean(c[k]))o[k]=c[k];});
  if(c.skips&&c.skips.length){const s=c.skips.filter(x=>clean(x.when)||clean(x.to));if(s.length)o.skips=s.map(x=>({when:x.when,to:x.to}));}
  if(c.validations&&c.validations.length){const v=c.validations.filter(x=>clean(x.test));if(v.length)o.validations=v.map(x=>({test:x.test,message:loc(x.message||""),severity:x.severity||"error"}));}
  return o;
}
function num(v){const n=Number(v);return Number.isFinite(n)?n:v;}
function coerce(v){if(v==="true")return true;if(v==="false")return false;if(v!==""&&!isNaN(Number(v)))return Number(v);return v;}

/* ===================== VALIDATION ===================== */
function runValidation(){const issues=lint();const errs=issues.filter(i=>i.sev==="error");const h=document.getElementById("health"),t=document.getElementById("healthTxt"),tn=document.getElementById("tabN");if(errs.length){h.className="health bad";t.textContent=`${errs.length} masalah`;tn.hidden=false;tn.textContent=errs.length;}else{h.className="health ok";t.textContent="Valid";tn.hidden=true;}renderJson(issues);}
function lint(){
  const issues=[],add=(sev,path,msg)=>issues.push({sev,path,msg});
  const names={},fields=new Set(),containers=new Set(),exprs=[],refs=[];
  const EK=["visibleWhen","enableWhen","requiredWhen","calculate"];const see=(n,p)=>{(names[n]=names[n]||[]).push(p);};
  function walk(n,base){const p=`${base}/${n.name||n.kind}`;
    if(n.kind==="field"){if(n.name){fields.add(n.name);see(n.name,p);}EK.forEach(k=>{if(clean(n[k]))exprs.push({path:`${p}.${k}`,expr:n[k]});});if(clean(n.optionsRef))refs.push({path:`${p}.optionsRef`,kind:"table",val:n.optionsRef});if(clean(n.optionsFilterBy))refs.push({path:`${p}.optionsFilterBy`,kind:"field",val:n.optionsFilterBy});(n.validations||[]).forEach((v,j)=>{if(clean(v.test))exprs.push({path:`${p}.val[${j}]`,expr:v.test});});(n.options||[]).forEach((o,j)=>{if(clean(o.skipTo))refs.push({path:`${p}.opsi[${j}]`,kind:"nav",val:o.skipTo});});(n.skips||[]).forEach((s,j)=>{if(clean(s.to))refs.push({path:`${p}.skip[${j}]`,kind:"nav",val:s.to});if(clean(s.when))exprs.push({path:`${p}.skip[${j}]`,expr:s.when});});}
    else{if(n.name){containers.add(n.name);see(n.name,p);}if(n.kind==="roster"&&clean(n.countFrom))refs.push({path:`${p}.countFrom`,kind:"field",val:n.countFrom});if(clean(n.visibleWhen))exprs.push({path:`${p}.visibleWhen`,expr:n.visibleWhen});(n.components||[]).forEach(c=>walk(c,p));}
  }
  state.pages.forEach(p=>walk(p,""));
  Object.entries(names).forEach(([n,ps])=>{if(ps.length>1)add("error",n,`Nama '${n}' dipakai ${ps.length}×`);});
  const tables=new Set(Object.keys(state.referenceData||{}));const nav=new Set([...fields,...containers,"__end","__next","__prev"]);
  refs.forEach(r=>{if(r.kind==="table"&&!tables.has(r.val))add("error",r.path,`optionsRef '${r.val}' tidak ada di referenceData`);if(r.kind==="field"&&!fields.has(r.val))add("error",r.path,`'${r.val}' bukan field yang ada`);if(r.kind==="nav"&&!nav.has(r.val))add("error",r.path,`target lompatan '${r.val}' tidak ditemukan`);});
  exprs.forEach(({path,expr})=>{try{Expr.parse(expr);}catch(e){add("error",path,"ekspresi tidak valid: "+e.message);}for(const m of String(expr).matchAll(/\$\{([^}]*)\}/g)){const b=m[1].trim().split(/[.\[]/)[0];if(!b||b.startsWith("__"))continue;if(!fields.has(b)&&!containers.has(b))add("error",path,`ekspresi merujuk '${b}' yang tidak ada`);}});
  const locs=new Set(state.locales||[]);if(state.defaultLocale&&locs.size&&!locs.has(state.defaultLocale))add("warning","defaultLocale",`locale utama '${state.defaultLocale}' tidak ada di locales`);
  return issues;
}
function renderJson(issues){const pane=document.getElementById("paneJson");const json=serialize();const errs=issues.filter(i=>i.sev==="error"),warns=issues.filter(i=>i.sev==="warning");let ih=!issues.length?`<div class="allgood">✓ Bersih — tidak ada masalah.</div>`:issues.map(it=>`<div class="issue ${it.sev==="error"?"err":"warn"}"><div><div>${esc(it.msg)}</div><div class="ipath">${esc(it.path)}</div></div></div>`).join("");pane.innerHTML=`<div class="jbar"><button class="btn" id="copyJson">Salin</button><button class="btn primary" id="dlJson">Unduh .json</button></div><pre class="json">${highlight(JSON.stringify(json,null,2))}</pre><div style="margin-top:14px"><div class="gh" style="font-size:11px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;color:var(--muted);margin-bottom:8px">Validasi — ${errs.length} error, ${warns.length} warning</div>${ih}</div>`;pane.querySelector("#copyJson").addEventListener("click",e=>{navigator.clipboard.writeText(JSON.stringify(json,null,2));e.target.textContent="Tersalin ✓";setTimeout(()=>e.target.textContent="Salin",1200);});pane.querySelector("#dlJson").addEventListener("click",()=>download(`${state.id||"kuesioner"}.json`,JSON.stringify(json,null,2)));}

/* ===================== UTIL ===================== */
function esc(s){return String(s??"").replace(/[&<>"]/g,m=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[m]));}
function opt(v,l,cur){return `<option value="${v}"${cur===v?" selected":""}>${l}</option>`;}
function reuid(n){n.uid=uid();(n.components||[]).forEach(reuid);}
function jsonOrEmpty(o){return (o&&Object.keys(o).length)?JSON.stringify(o,null,2):"";}
function download(name,text){try{const b=new Blob([text],{type:"application/json"});const a=document.createElement("a");a.href=URL.createObjectURL(b);a.download=name;a.click();URL.revokeObjectURL(a.href);}catch(_){}}
function highlight(j){return esc(j).replace(/&quot;([^&]+?)&quot;(\s*:)/g,'<span class="jk">&quot;$1&quot;</span>$2').replace(/:\s*&quot;([^&]*?)&quot;/g,': <span class="js">&quot;$1&quot;</span>').replace(/:\s*(-?\d+\.?\d*)/g,': <span class="jn">$1</span>').replace(/:\s*(true|false|null)/g,': <span class="jb">$1</span>');}

/* ===================== IMPORT ===================== */
function importJSON(obj){try{
  const st=blankState();st.pages=[];
  st.id=obj.id||st.id;st.version=obj.version||st.version;st.acronym=obj.acronym||"";st.title=textOf(obj.title)||st.title;st.locales=obj.locales||["id"];st.defaultLocale=obj.defaultLocale||st.defaultLocale;
  if(obj.settings){st.settings=Object.assign(st.settings,obj.settings);st.settings.navigation=Object.assign(blankState().settings.navigation,obj.settings.navigation||{});}
  st.referenceData=obj.referenceData||{};
  const pages = obj.pages || (obj.sections? obj.sections.map(s=>({kind:"page",name:s.name,title:s.title,visibleWhen:s.visibleWhen,components:s.components})) : []);
  pages.forEach(p=>st.pages.push(impNode(p,"page")));
  if(!st.pages.length)st.pages.push(blankState().pages[0]);
  state=st;selected=null;view={type:"page",uid:state.pages[0].uid};render();
}catch(e){alert("Gagal impor: "+e.message);}}
function impNode(n,forceKind){
  const kind=forceKind||n.kind||"field";
  if(kind==="page"||kind==="block"||kind==="section"){return {uid:uid(),kind,name:n.name||autoName(kind),title:textOf(n.title),visibleWhen:n.visibleWhen||"",components:(n.components||[]).map(c=>impNode(c))};}
  if(kind==="roster"){return {uid:uid(),kind:"roster",name:n.name||autoName("roster"),title:textOf(n.title),rowTitle:n.rowTitle||"",rosterType:n.rosterType||"inline",min:n.min??"",max:n.max??"",countFrom:n.countFrom||"",itemLabel:textOf(n.itemLabel),rowDisplay:n.rowDisplay||[],visibleWhen:n.visibleWhen||"",components:(n.components||[]).map(c=>impNode(c))};}
  const f=newField(n.type||"text");f.uid=uid();f.name=n.name||f.name;f.label=textOf(n.label);f.hint=textOf(n.hint);f.html=textOf(n.html);f.markdown=textOf(n.markdown);f.calculate=n.calculate||"";
  ["required","readOnly","allowRemark","visibleWhen","enableWhen","requiredWhen","unit","pattern","optionsRef","optionsFilterBy","min","max","step","maxLength"].forEach(k=>{if(n[k]!=null)f[k]=n[k];});
  f.placeholder=textOf(n.placeholder);
  if(n.options)f.options=n.options.map(o=>({value:String(o.value),label:textOf(o.label),skipTo:o.skipTo||""}));
  if(n.optionsApi){f.optionsApi={...n.optionsApi};f.optionSource="api";}else if(n.optionsRef){f.optionSource="ref";}else if(CHOICE.has(f.type))f.optionSource="manual";
  if(n.skips)f.skips=n.skips.map(s=>({when:s.when||"",to:s.to||""}));
  if(n.validations)f.validations=n.validations.map(v=>({test:v.test||"",message:textOf(v.message),severity:v.severity||"error"}));
  return f;
}
function textOf(v){if(v==null)return "";if(typeof v==="string")return v;if(typeof v==="object")return v[state?.defaultLocale]||v.id||Object.values(v)[0]||"";return String(v);}

/* ===================== TABS / COLLAPSE / TOP ACTIONS ===================== */
function switchTab(name){document.querySelectorAll(".tab").forEach(t=>t.classList.toggle("on",t.dataset.tab===name));document.getElementById("paneProps").hidden=name!=="props";document.getElementById("paneJson").hidden=name!=="json";}
function applyCols(){const c1=collapsed.sb1?"46px":"212px";const c2=collapsed.sb2?"46px":"226px";document.getElementById("cols").style.gridTemplateColumns=`${c1} ${c2} 1fr 330px`;document.getElementById("sb1").classList.toggle("collapsed",collapsed.sb1);document.getElementById("sb2").classList.toggle("collapsed",collapsed.sb2);}
document.querySelectorAll(".tab").forEach(t=>t.addEventListener("click",()=>switchTab(t.dataset.tab)));
document.getElementById("health").addEventListener("click",()=>switchTab("json"));
document.getElementById("col1").addEventListener("click",()=>{collapsed.sb1=true;applyCols();});
document.getElementById("exp1").addEventListener("click",()=>{collapsed.sb1=false;applyCols();});
document.getElementById("col2").addEventListener("click",()=>{collapsed.sb2=true;applyCols();});
document.getElementById("exp2").addEventListener("click",()=>{collapsed.sb2=false;applyCols();});
document.getElementById("instTitle").addEventListener("input",e=>{state.title=e.target.value;runValidation();});
document.getElementById("addPage").addEventListener("click",()=>{const p=newPage();p.title="Halaman "+(state.pages.length+1);state.pages.push(p);view={type:"page",uid:p.uid};selected=p.uid;render();});
document.getElementById("btnExport").addEventListener("click",()=>{switchTab("json");download(`${state.id||"kuesioner"}.json`,JSON.stringify(serialize(),null,2));});
document.getElementById("btnImport").addEventListener("click",()=>document.getElementById("fileInput").click());
document.getElementById("fileInput").addEventListener("change",e=>{const f=e.target.files[0];if(!f)return;const r=new FileReader();r.onload=()=>{try{importJSON(JSON.parse(r.result));}catch(err){alert("JSON tidak valid: "+err.message);}};r.readAsText(f);e.target.value="";});
document.getElementById("btnExample").addEventListener("click",()=>importJSON(EXAMPLE));
document.getElementById("btnPreview").addEventListener("click",openPreview);
document.getElementById("pvClose").addEventListener("click",closePreview);
document.getElementById("pvMode").addEventListener("change",e=>{pv.mode=e.target.value;pv.page=0;renderPreview();});
document.addEventListener("keydown",e=>{
  if(e.key==="Escape"&&!document.getElementById("preview").hidden){closePreview();return;}
  const at=document.activeElement,tag=at&&at.tagName;
  if(tag==="INPUT"||tag==="TEXTAREA"||(at&&at.isContentEditable))return; // jangan ganggu copy-paste teks biasa
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="c"&&selected){const n=findNode(selected);if(n){copyNode(n);render();e.preventDefault();}}
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="v"&&clipboard){pasteNode();e.preventDefault();}
});
document.querySelector(".canvas").addEventListener("click",e=>{if(e.target.classList.contains("canvas")||e.target.closest(".cv-head")&&!e.target.closest("button")){selected=null;render();}});

/* ===================== PREVIEW (Lihat Kuesioner) ===================== */
let pv={values:{},page:0,mode:"section",row:null};
function openPreview(){pv={values:{},page:0,mode:state.settings.navigation.mode==="scroll"?"scroll":"section",row:null,apiCache:{}};const sel=document.getElementById("pvMode");if(sel)sel.value=pv.mode;document.getElementById("preview").hidden=false;renderPreview();}
function closePreview(){document.getElementById("preview").hidden=true;}
function coerceVal(v){if(v==="true")return true;if(v==="false")return false;if(typeof v==="string"&&v.trim()!==""&&!isNaN(Number(v)))return Number(v);return v;}
function refResolve(name,rowPrefix){
  name=String(name).trim();
  if(name.includes(".")){const [rn,fn]=name.split(".");const r=allNodes().find(x=>x.kind==="roster"&&x.name===rn);if(r){const cnt=rosterCount(r);const arr=[];for(let i=0;i<cnt;i++)arr.push(coerceVal(pv.values[`${rn}#${i}#${fn}`]));return arr;}return undefined;}
  const rn=allNodes().find(x=>x.kind==="roster"&&x.name===name);
  if(rn){const cnt=rosterCount(rn);return Array.from({length:cnt},(_,i)=>i);}
  if(rowPrefix){const k=rowPrefix+name;if(k in pv.values)return coerceVal(pv.values[k]);}
  return coerceVal(pv.values[name]);
}
function evalExprSrc(src,rowPrefix){return Expr.evalSrc(src,name=>refResolve(name,rowPrefix||""));}
function evalVisible(src,rowPrefix){if(!src)return true;const v=evalExprSrc(src,rowPrefix||"");return v===undefined?true:!!v;}
function pvEmpty(v){return v==null||v===""||(Array.isArray(v)&&v.length===0);}
function refLabels(ref,parentVal,filterField){const tbl=state.referenceData&&state.referenceData[ref];if(!tbl||!tbl.items)return [];return tbl.items.filter(it=>{if(filterField&&parentVal!=null&&parentVal!=="")return String(it.parent)===String(parentVal);return true;}).map(it=>({value:it.code,label:textOf(it.label)}));}
function pvOptions(c,rowPrefix){if(c.optionsRef){const pVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rowPrefix):null;return refLabels(c.optionsRef,pVal,c.optionsFilterBy);}return (c.options||[]).map(o=>({value:o.value,label:textOf(o.label)||String(o.value)}));}
function getPath(obj,path){return String(path).split(".").reduce((o,k)=>(o==null?o:o[k]),obj);}
function buildApiUrl(tbl,parentVal){
  let url=tbl.url;
  // Ganti semua {key}: {parent} → parentVal, {key} lain → pv.values[key]
  url=url.replace(/\{([^}]+)\}/g,(_,k)=>encodeURIComponent(k==="parent"?(parentVal??""): (pv.values[k]??"")));
  // Fallback: URL tanpa placeholder, tapi ada parentParam → append sebagai query
  if(!tbl.url.includes("{")&&tbl.parentParam&&parentVal!=null&&parentVal!=="")
    url+=(url.includes("?")?"&":"?")+encodeURIComponent(tbl.parentParam)+"="+encodeURIComponent(parentVal);
  return url;
}
function apiFetch(tbl,parentVal){
  pv.apiCache=pv.apiCache||{};const url=buildApiUrl(tbl,parentVal);let e=pv.apiCache[url];
  if(!e){e=pv.apiCache[url]={state:"loading",opts:[]};
    fetch(url,{method:tbl.method||"GET",headers:tbl.headers||{}})
      .then(r=>{if(!r.ok)throw new Error("HTTP "+r.status);return r.json();})
      .then(data=>{let arr=tbl.path?getPath(data,tbl.path):(Array.isArray(data)?data:(data.data||data.items||data.results||[]));if(!Array.isArray(arr))arr=[];const vf=tbl.valueField||"code",lf=tbl.labelField||"label";e.opts=arr.map(it=>({value:it[vf],label:textOf(it[lf])??String(it[vf])}));e.state="done";if(!document.getElementById("preview").hidden)renderPreview();})
      .catch(err=>{e.state="error";e.error=String(err.message||err);if(!document.getElementById("preview").hidden)renderPreview();});
  }
  return e;
}
function resolveOptions(c,rp){
  const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
  if(mode==="api"){
    const cfg=c.optionsApi||{};if(!cfg.url)return {state:"ok",opts:[]};
    // Dependency: placeholder {key} di URL (selain {parent}) + dataKey trigger eksplisit (depKeys)
    const urlDeps=(cfg.url.match(/\{([^}]+)\}/g)||[]).map(m=>m.slice(1,-1)).filter(k=>k!=="parent");
    const explicitDeps=(cfg.depKeys||"").split(",").map(s=>s.trim()).filter(Boolean);
    const deps=[...new Set([...urlDeps,...explicitDeps])];
    const missing=deps.find(k=>pv.values[k]==null||pv.values[k]==="");
    if(missing)return {state:"skip",opts:[]};
    const parentVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rp):null;
    const e=apiFetch(cfg,parentVal);return {state:e.state==="done"?"ok":e.state,opts:e.opts||[],error:e.error};
  }
  if(mode==="ref"&&c.optionsRef){const tbl=state.referenceData&&state.referenceData[c.optionsRef];if(!tbl)return {state:"ok",opts:[]};const parentVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rp):null;if(tbl.source==="api"){const e=apiFetch(tbl,parentVal);return {state:e.state==="done"?"ok":e.state,opts:e.opts||[],error:e.error};}return {state:"ok",opts:refLabels(c.optionsRef,parentVal,c.optionsFilterBy)};}
  return {state:"ok",opts:(c.options||[]).map(o=>({value:o.value,label:textOf(o.label)||String(o.value)}))};
}
function optWrap(ro,fn,key){
  if(ro.state==="skip"){if(key!=null)pv.values[key]="";return '<select class="pv-in" disabled><option value="">— isi kolom sebelumnya dahulu —</option></select>';}
  if(ro.state==="loading")return '<div class="pv-loading">⏳ Memuat pilihan dari API…</div>';
  if(ro.state==="error")return `<div class="pv-vmsg error">Gagal memuat pilihan: ${esc(ro.error||"")}</div>`;
  return fn();
}
function visiblePages(){return state.pages.filter(p=>evalVisible(p.visibleWhen,""));}
function nodeContainsName(node,target){if(node.name===target)return true;return (node.components||[]).some(c=>nodeContainsName(c,target));}
function pageIndexOfTarget(target,pages){
  if(!target)return null;
  if(target==="__next")return null;
  if(target==="__prev")return pv.page>0?pv.page-1:null;
  if(target==="__end")return pages.length-1;
  for(let i=0;i<pages.length;i++){if(nodeContainsName(pages[i],target))return i;}
  return null;
}
function fieldSkipTarget(f){
  for(const s of (f.skips||[])){
    if(!s.when||!s.to)continue;
    const r=evalExprSrc(s.when,"");
    if(r===undefined)continue;
    if(r)return s.to;
  }
  if(CHOICE.has(f.type)&&Array.isArray(f.options)){
    const v=pv.values[f.name];
    const sel=Array.isArray(v)?v.map(String):[String(v)];
    for(const o of f.options){if(o.skipTo&&sel.includes(String(o.value)))return o.skipTo;}
  }
  return null;
}
// Target skip pertama yang terdaftar pada field ini (dipakai sebagai default
// ketika field belum dijawab sama sekali — lihat fieldEffectiveSkipTarget).
function fieldPendingTarget(f){
  const s=(f.skips||[]).find(x=>x.when&&x.to);
  if(s)return s.to;
  if(CHOICE.has(f.type)&&Array.isArray(f.options)){
    const o=f.options.find(o=>o.skipTo);
    if(o)return o.skipTo;
  }
  return null;
}
// Field yang punya kemampuan skip tapi BELUM dijawab dianggap "pending":
// secara default field di antara dia dan target tetap disembunyikan sampai
// dijawab dengan opsi yang TIDAK memicu skip (baru jalur normal terbuka).
function fieldEffectiveSkipTarget(f){
  const t=fieldSkipTarget(f);
  if(t)return t;
  if(pvEmpty(pv.values[f.name]))return fieldPendingTarget(f);
  return null;
}
// Hitung field di halaman ini yang harus disembunyikan karena skip yang
// sedang aktif (target di halaman yang sama), plus target lintas-halaman
// bila skip yang aktif belum "selesai" sampai akhir halaman.
function computePageSkipState(page){
  const hidden=new Set();
  const fields=[];
  (function walk(n,prefix){(n.components||[]).forEach(c=>{if(c.kind==="field")fields.push(c);else if(c.kind!=="roster")walk(c,prefix);});})(page,"");
  let skipActive=false,skipTarget=null,crossPageTarget=null;
  for(const f of fields){
    if(skipActive){
      if(f.name===skipTarget){skipActive=false;skipTarget=null;}
      else{hidden.add(f.name);continue;}
    }
    const t=fieldEffectiveSkipTarget(f);
    if(t&&t!=="__next"){
      if(nodeContainsName(page,t)&&t!==f.name){skipActive=true;skipTarget=t;}
      else{skipActive=true;skipTarget=null;crossPageTarget=t;}
    }
  }
  return{hidden,crossPageTarget:skipActive?crossPageTarget:null};
}
let SKIP_HIDDEN=new Set();
function clearPageValues(page){
  (function walk(n,prefix){(n.components||[]).forEach(c=>{
    if(c.kind==="field"){delete pv.values[prefix+c.name];}
    else if(c.kind==="roster"){
      const cnt=Math.max(rosterCount(c),Number(pv.values[`${c.name}#count`])||0);
      for(let i=0;i<cnt;i++){(c.components||[]).forEach(f=>{delete pv.values[`${c.name}#${i}#${f.name}`];});}
      delete pv.values[`${c.name}#count`];
    }else{walk(c,prefix);}
  });})(page,"");
}
function clearSkippedPages(pages,fromIdx,toIdx){if(toIdx<=fromIdx+1)return;for(let i=fromIdx+1;i<toIdx;i++)clearPageValues(pages[i]);}
function pageValidationTargets(page){
  const out=[];
  (function walk(n,prefix){(n.components||[]).forEach(c=>{
    if(!evalVisible(c.visibleWhen,prefix))return;
    if(c.kind==="field"){if(!SKIP_HIDDEN.has(c.name))out.push({c,rp:prefix});}
    else if(c.kind==="roster"){
      if(c.rosterType==="inline"){
        const cnt=rosterCount(c);
        for(let i=0;i<cnt;i++){const rp2=`${c.name}#${i}#`;(c.components||[]).forEach(f=>{if(evalVisible(f.visibleWhen,rp2))out.push({c:f,rp:rp2});});}
      }
    }else{walk(c,prefix);}
  });})(page,"");
  return out;
}
function validateCurrentPage(page){
  const targets=pageValidationTargets(page);
  for(const {c,rp} of targets){
    if(c.type==="note"||c.type==="markdown"||c.type==="hidden"||c.type==="calculated")continue;
    if(!evalVisible(c.enableWhen,rp))continue;
    const key=rp+c.name,val=pv.values[key];
    const isRequired=!!c.required||!!(c.requiredWhen&&evalVisible(c.requiredWhen,rp));
    if(isRequired&&pvEmpty(val))return{ok:false,key};
    if(!pvEmpty(val)){
      for(const v of (c.validations||[])){
        if(!v.test||v.severity==="warning")continue;
        const r=evalExprSrc(v.test,rp);
        if(r===undefined)continue;
        if(!r)return{ok:false,key};
      }
    }
  }
  return{ok:true,key:null};
}
function focusPvField(key){
  const el=document.querySelector(`[data-fieldkey="${CSS.escape(key)}"]`);
  if(!el)return;
  el.classList.add("pv-field-err");
  el.scrollIntoView({behavior:"smooth",block:"center"});
  setTimeout(()=>el.classList.remove("pv-field-err"),2500);
}
function renderPreview(){
  const body=document.getElementById("pvBody");const keep=body.scrollTop;
  document.getElementById("pvTitle").textContent=textOf(state.title)||"Kuesioner";
  if(pv.row){renderRosterRowPage();renderPvSide();return;}
  const pages=visiblePages();
  if(!pages.length){body.innerHTML=`<div class="pv-empty">Belum ada halaman untuk ditampilkan.</div>`;renderPvSide();return;}
  let html="";
  if(pv.mode==="scroll"){SKIP_HIDDEN=new Set();pages.forEach(p=>html+=pvPage(p));}
  else{if(pv.page>=pages.length)pv.page=pages.length-1;const p=pages[pv.page];
    SKIP_HIDDEN=computePageSkipState(p).hidden;
    SKIP_HIDDEN.forEach(name=>{delete pv.values[name];});
    html+=pvPage(p);
    html+=`<div class="pv-nav-err" id="pvNavErr"></div><div class="pv-nav">${pv.page>0?`<button class="btn" id="pvPrev">← Sebelumnya</button>`:`<span></span>`}<span class="pv-prog">Halaman ${pv.page+1} / ${pages.length}</span>${pv.page<pages.length-1?`<button class="btn primary" id="pvNext">Lanjut →</button>`:`<button class="btn primary" id="pvDone">Selesai</button>`}</div>`;}
  body.innerHTML=html;bindPreview(body);body.scrollTop=keep;renderPvSide();
  body.querySelector("#pvPrev")?.addEventListener("click",()=>{pv.page--;body.scrollTop=0;renderPreview();});
  body.querySelector("#pvNext")?.addEventListener("click",()=>{
    const curP=pages[pv.page];
    const gate=validateCurrentPage(curP);
    if(!gate.ok){const err=document.getElementById("pvNavErr");if(err)err.textContent="Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum melanjutkan.";focusPvField(gate.key);return;}
    const target=computePageSkipState(curP).crossPageTarget;
    const idx=target?pageIndexOfTarget(target,pages):null;
    if(idx!=null){clearSkippedPages(pages,pv.page,idx);pv.page=idx;body.scrollTop=0;renderPreview();return;}
    pv.page++;body.scrollTop=0;renderPreview();
  });
  body.querySelector("#pvDone")?.addEventListener("click",()=>{
    const curP=pages[pv.page];
    const gate=validateCurrentPage(curP);
    if(!gate.ok){const err=document.getElementById("pvNavErr");if(err)err.textContent="Lengkapi pertanyaan wajib / perbaiki isian yang tidak valid sebelum mengirim.";focusPvField(gate.key);return;}
    alert("Preview selesai. Ini hanya tampilan — data tidak disimpan.");
  });
}
function pvPage(p){let h=`<div class="pv-page" id="pvpage_${esc(p.name)}"><h2 class="pv-h2">${esc(p.title||p.name)}</h2>`;p.components.forEach(c=>h+=pvNode(c,null));return h+`</div>`;}
function pvNode(c,row){
  const rp=row?`${row.r}#${row.i}#`:"";
  if(c.kind==="block"){if(!evalVisible(c.visibleWhen,rp))return "";let h=`<div class="pv-card">`;if(c.title)h+=`<div class="pv-bt">${esc(c.title)}</div>`;c.components.forEach(x=>h+=pvNode(x,row));return h+`</div>`;}
  if(c.kind==="section"){if(!evalVisible(c.visibleWhen,rp))return "";let h=`<div class="pv-sec">`;if(c.title)h+=`<div class="pv-st">${esc(c.title)}</div>`;c.components.forEach(x=>h+=pvNode(x,row));return h+`</div>`;}
  if(c.kind==="roster"){if(!evalVisible(c.visibleWhen,rp))return "";return pvRoster(c);}
  return pvField(c,row);
}
function rosterCount(r){
  if(r.countFrom){let cf=Number(pv.values[r.countFrom]);if(!Number.isFinite(cf)||cf<0)cf=0;cf=Math.floor(cf);if(r.max!==""&&r.max!=null&&cf>Number(r.max))cf=Number(r.max);return cf;}
  const k=`${r.name}#count`;if(pv.values[k]==null)pv.values[k]=Number(r.min)||1;return pv.values[k];
}
function labelOfField(name){const n=allNodes().find(x=>x.kind==="field"&&x.name===name);return n?(n.label||n.name):name;}
function rowSummary(r,i){
  const disp=(r.rowDisplay&&r.rowDisplay.length)?r.rowDisplay:((r.components||[]).filter(c=>c.kind==="field").slice(0,1).map(c=>c.name));
  const parts=disp.map(fn=>{const v=pv.values[`${r.name}#${i}#${fn}`];return (v==null||v==="")?null:String(v);}).filter(Boolean);
  return parts.length?esc(parts.join(" · ")):"";
}
function isRowFilled(r,i){return (r.components||[]).some(f=>{const v=pv.values[`${r.name}#${i}#${f.name}`];return v!=null&&v!=="";});}
function primaryRowField(r){if(r.rowDisplay&&r.rowDisplay.length)return r.rowDisplay[0];const f=(r.components||[]).find(c=>c.kind==="field");return f?f.name:null;}
function openAddRowModal(r){
  const title=r.rowTitle||"baris";
  const bg=document.createElement("div");bg.className="pv-modal-bg";
  bg.innerHTML=`<div class="pv-modal">
    <h3>Tambah ${esc(title)}</h3>
    <p class="pv-modal-sub">Masukkan nama/judul untuk ${esc(title)} ini.</p>
    <input type="text" id="addRowInput" placeholder="${esc(title)}…">
    <div class="pv-modal-actions"><button class="btn ghost" id="addRowCancel">Batal</button><button class="btn primary" id="addRowConfirm">+ Tambah ${esc(title)}</button></div>
  </div>`;
  document.body.appendChild(bg);
  const input=bg.querySelector("#addRowInput");input.focus();
  const close=()=>bg.remove();
  bg.querySelector("#addRowCancel").addEventListener("click",close);
  bg.addEventListener("click",e=>{if(e.target===bg)close();});
  const confirmAdd=()=>{
    const val=input.value.trim();
    const idx=pv.values[`${r.name}#count`]||0;
    pv.values[`${r.name}#count`]=idx+1;
    if(val){const pf=primaryRowField(r);if(pf)pv.values[`${r.name}#${idx}#${pf}`]=val;}
    close();renderPreview();
  };
  bg.querySelector("#addRowConfirm").addEventListener("click",confirmAdd);
  input.addEventListener("keydown",e=>{if(e.key==="Enter"){e.preventDefault();confirmAdd();}if(e.key==="Escape")close();});
}
function delRow(rname,idx){
  const key=`${rname}#count`;const count=pv.values[key]||1;const prefix=`${rname}#`;const nv={};
  Object.keys(pv.values).forEach(k=>{
    if(k===key)return;
    if(k.startsWith(prefix)){const m=k.slice(prefix.length).match(/^(\d+)#(.*)$/);if(m){let ri=+m[1];const fn=m[2];if(ri===idx)return;if(ri>idx)ri--;nv[`${prefix}${ri}#${fn}`]=pv.values[k];return;}}
    nv[k]=pv.values[k];
  });
  nv[key]=Math.max(0,count-1);pv.values=nv;
}
function addRowLabel(r){return r.rowTitle?`+ Tambah ${esc(r.rowTitle)}`:"+ Tambah baris";}
function rowLabel(r,i){return r.rowTitle?`${esc(r.rowTitle)} #${i+1}`:`Baris #${i+1}`;}
function pvRoster(r){
  const count=rosterCount(r);const manual=!r.countFrom;
  if(r.rosterType==="separate"){
    let h=`<div class="pv-roster" id="pvroster_${esc(r.name)}"><div class="pv-rh">${esc(r.title||r.name)}<span class="pv-tag">subhalaman</span></div>`;
    if(r.countFrom&&count<=0){h+=`<div class="pv-rowempty">Isi dulu “${esc(labelOfField(r.countFrom))}” untuk menentukan jumlah baris.</div>`;}
    else if(count<=0){h+=`<div class="pv-rowempty">Belum ada baris.</div>`;}
    else{h+=`<div class="pv-rowlist">`;
      for(let i=0;i<count;i++){const sum=rowSummary(r,i);
        h+=`<div class="pv-rowitem"><div class="pv-rowinfo"><b>${rowLabel(r,i)}</b><span>${sum||"<i>belum diisi</i>"}</span></div>${manual?`<button class="pv-rowdel" data-delrow="${esc(r.name)}" data-i="${i}">hapus</button>`:""}<button class="pv-rowopen" data-openrow="${r.uid}" data-i="${i}">${isRowFilled(r,i)?"Edit":"Isi"} →</button></div>`;}
      h+=`</div>`;}
    if(manual)h+=`<button class="pv-add" data-addrow="${esc(r.uid)}">${addRowLabel(r)}</button>`;
    return h+`</div>`;
  }
  // inline
  let h=`<div class="pv-roster" id="pvroster_${esc(r.name)}"><div class="pv-rh">${esc(r.title||r.name)}</div>`;
  if(r.countFrom&&count<=0)h+=`<div class="pv-rowempty">Isi dulu “${esc(labelOfField(r.countFrom))}” untuk menentukan jumlah baris.</div>`;
  for(let i=0;i<count;i++){h+=`<div class="pv-row"><div class="pv-rownum"><span>${rowLabel(r,i)}</span>${manual?`<button class="pv-del" data-delrow="${esc(r.name)}" data-i="${i}">hapus</button>`:""}</div>`;r.components.forEach(f=>h+=pvNode(f,{r:r.name,i}));h+=`</div>`;}
  if(manual)h+=`<button class="pv-add" data-addrow="${esc(r.uid)}">${addRowLabel(r)}</button>`;
  return h+`</div>`;
}
function renderRosterRowPage(){
  const body=document.getElementById("pvBody");const keep=body.scrollTop;
  const r=findNode(pv.row.uid);if(!r){pv.row=null;return renderPreview();}
  const i=pv.row.index;const parent=pageOf(r.uid);
  let h=`<div class="pv-page"><button class="btn" id="pvBack" style="margin-bottom:14px">← ${esc(parent?(parent.title||parent.name):"Kembali")}</button><h2 class="pv-h2">${esc(r.title||r.name)} — ${rowLabel(r,i)}</h2>`;
  r.components.forEach(f=>h+=pvNode(f,{r:r.name,i}));
  h+=`<div class="pv-nav"><span></span><button class="btn primary" id="pvBackDone">Simpan baris &amp; kembali</button></div></div>`;
  body.innerHTML=h;bindPreview(body);body.scrollTop=keep;
  body.querySelector("#pvBack")?.addEventListener("click",backFromRow);
  body.querySelector("#pvBackDone")?.addEventListener("click",backFromRow);
}
function backFromRow(){
  const r=findNode(pv.row.uid);const parent=r?pageOf(r.uid):null;const rname=r?r.name:"";pv.row=null;
  if(parent&&pv.mode==="section"){const idx=visiblePages().indexOf(parent);if(idx>=0)pv.page=idx;}
  renderPreview();setTimeout(()=>{const el=document.getElementById("pvroster_"+rname);if(el)el.scrollIntoView({block:"center"});},40);
}
function pvField(c,row){
  if(c.type==="hidden")return "";
  const rp=row?`${row.r}#${row.i}#`:"";
  if(!evalVisible(c.visibleWhen,rp))return "";
  if(!row&&SKIP_HIDDEN.has(c.name))return ""; // disembunyikan oleh skip-to field sebelumnya di halaman ini
  if(c.type==="note")return `<div class="pv-note">${c.html||""}</div>`;
  if(c.type==="markdown")return `<div class="pv-note pv-md">${mdToHtml(c.markdown||"")}</div>`;
  const key=row?`${row.r}#${row.i}#${c.name}`:c.name;
  if(c.type==="calculated"){const r=evalExprSrc(c.calculate,rp);pv.values[key]=(r===undefined?"":r);const lab=`<label class="pv-lab">${esc(c.label||c.name)}</label>`;const hint=c.hint?`<div class="pv-hint">${esc(c.hint)}</div>`:"";return `<div class="pv-field" data-fieldkey="${esc(key)}">${lab}${hint}<div><input class="pv-in" value="${esc(r===undefined||r===""?"—":String(r))}" disabled></div></div>`;}
  const val=pv.values[key]??"";
  const isRequired=!!c.required||!!(c.requiredWhen&&evalVisible(c.requiredWhen,rp));
  const enabled=evalVisible(c.enableWhen,rp);
  const dis=enabled?"":" disabled";
  const lab=`<label class="pv-lab">${esc(c.label||c.name)}${isRequired?' <span class="pv-req">*</span>':''}</label>`;
  const hint=c.hint?`<div class="pv-hint">${esc(c.hint)}</div>`:"";
  const da=`data-k="${esc(key)}"`;let ctrl="";
  if(c.type==="textarea")ctrl=`<textarea ${da} class="pv-in" rows="3"${dis}>${esc(val)}</textarea>`;
  else if(c.type==="text")ctrl=`<input ${da} class="pv-in" value="${esc(val)}" placeholder="${esc(c.placeholder||"")}"${dis}>`;
  else if(NUMERIC.has(c.type))ctrl=`<input ${da} type="number" class="pv-in" value="${esc(val)}"${c.min!==""&&c.min!=null?` min="${c.min}"`:""}${c.max!==""&&c.max!=null?` max="${c.max}"`:""}${dis} style="width:auto;min-width:160px">${c.unit?`<span class="pv-unit">${esc(c.unit)}</span>`:""}`;
  else if(c.type==="boolean")ctrl=pvRadios(key,[{value:"true",label:"Ya"},{value:"false",label:"Tidak"}],String(val),dis);
  else if(c.type==="radio"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>pvRadios(key,ro.opts,String(val),dis),key);}
  else if(c.type==="select"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>`<select ${da} class="pv-in"${dis}><option value="">— pilih —</option>${ro.opts.map(o=>`<option value="${esc(o.value)}"${String(val)===String(o.value)?" selected":""}>${esc(o.label)}</option>`).join("")}</select>`,key);}
  else if(c.type==="checkbox"||c.type==="multiselect"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>`<div class="pv-radios">${ro.opts.map(o=>`<label class="pv-opt"><input type="checkbox" data-kc="${esc(key)}" value="${esc(o.value)}"${(Array.isArray(val)&&val.map(String).includes(String(o.value)))?" checked":""}${dis}> ${esc(o.label)}</label>`).join("")}</div>`,key);}
  else if(c.type==="date")ctrl=`<input ${da} type="date" class="pv-in" value="${esc(val)}" style="width:auto"${dis}>`;
  else if(c.type==="time")ctrl=`<input ${da} type="time" class="pv-in" value="${esc(val)}" style="width:auto"${dis}>`;
  else if(c.type==="datetime")ctrl=`<input ${da} type="datetime-local" class="pv-in" value="${esc(val)}" style="width:auto"${dis}>`;
  else if(c.type==="geopoint")ctrl=`<input ${da} class="pv-in" value="${esc(val)}" placeholder="lat, lng"${dis}>`;
  else if(c.type==="photo"||c.type==="file")ctrl=`<input type="file" class="pv-in"${c.type==="photo"?' accept="image/*"':''}${dis}>`;
  else if(c.type==="signature")ctrl=`<div class="pv-sign">Area tanda tangan</div>`;
  else if(c.type==="barcode")ctrl=`<input ${da} class="pv-in" value="${esc(val)}" placeholder="scan / ketik kode"${dis}>`;
  else ctrl=`<input ${da} class="pv-in" value="${esc(val)}"${dis}>`;
  let vmsg="";
  (c.validations||[]).forEach(v=>{if(!v.test)return;if(pvEmpty(val))return;const r=evalExprSrc(v.test,rp);if(r===undefined)return;if(!r)vmsg+=`<div class="pv-vmsg ${v.severity==="warning"?"warning":"error"}">${esc(textOf(v.message)||"Nilai tidak valid")}</div>`;});
  return `<div class="pv-field" data-fieldkey="${esc(key)}">${lab}${hint}<div>${ctrl}</div>${vmsg}</div>`;
}
function pvRadios(key,opts,val,dis){dis=dis||"";return `<div class="pv-radios">${opts.map(o=>`<label class="pv-opt"><input type="radio" name="r_${esc(key)}" data-kr="${esc(key)}" value="${esc(o.value)}"${String(val)===String(o.value)?" checked":""}${dis}> ${esc(o.label)}</label>`).join("")}</div>`;}
function bindPreview(body){
  body.querySelectorAll("[data-k]").forEach(inp=>{const key=inp.getAttribute("data-k");inp.addEventListener("input",()=>{pv.values[key]=inp.value;});inp.addEventListener("change",()=>{pv.values[key]=inp.value;renderPreview();});});
  body.querySelectorAll("[data-kr]").forEach(inp=>inp.addEventListener("change",()=>{pv.values[inp.getAttribute("data-kr")]=inp.value;renderPreview();}));
  body.querySelectorAll("[data-kc]").forEach(inp=>inp.addEventListener("change",()=>{const key=inp.getAttribute("data-kc");const arr=Array.isArray(pv.values[key])?pv.values[key]:[];const v=inp.value;if(inp.checked){if(!arr.includes(v))arr.push(v);}else{const idx=arr.indexOf(v);if(idx>=0)arr.splice(idx,1);}pv.values[key]=arr;renderPreview();}));
  body.querySelectorAll("[data-addrow]").forEach(b=>b.addEventListener("click",()=>{const r=findNode(b.getAttribute("data-addrow"));if(r)openAddRowModal(r);}));
  body.querySelectorAll("[data-delrow]").forEach(b=>b.addEventListener("click",()=>{delRow(b.getAttribute("data-delrow"),+b.getAttribute("data-i"));renderPreview();}));
  body.querySelectorAll("[data-openrow]").forEach(b=>b.addEventListener("click",()=>{pv.row={uid:b.getAttribute("data-openrow"),index:+b.getAttribute("data-i")};document.getElementById("pvBody").scrollTop=0;renderPreview();}));
}

function renderPvSide(){
  const side=document.getElementById("pvSide");if(!side)return;
  const pages=visiblePages();
  let activeIdx=-1,activeRosterUid=null;
  if(pv.row){const rn=findNode(pv.row.uid);activeRosterUid=pv.row.uid;const pp=rn&&pageOf(rn.uid);activeIdx=pp?pages.indexOf(pp):-1;}
  else if(pv.mode==="section")activeIdx=pv.page;
  let html=`<div class="pv-side-h">Daftar Halaman</div>`;
  pages.forEach((p,idx)=>{
    html+=`<div class="pv-pg${idx===activeIdx&&!activeRosterUid?" active":""}" data-pvpage="${idx}">${esc(p.title||p.name)}</div>`;
    separateRosters(p).forEach(r=>{html+=`<div class="pv-subpg${r.uid===activeRosterUid?" active":""}" data-pvroster="${esc(r.name)}" data-pvidx="${idx}"><span class="ri">⊞</span><span>${esc(r.title||r.name)}</span></div>`;});
  });
  side.innerHTML=html;
  side.querySelectorAll("[data-pvpage]").forEach(el=>el.addEventListener("click",()=>goToPage(+el.dataset.pvpage)));
  side.querySelectorAll("[data-pvroster]").forEach(el=>el.addEventListener("click",()=>goToPage(+el.dataset.pvidx,"pvroster_"+el.dataset.pvroster)));
}
function goToPage(idx,scrollToId){
  pv.row=null;
  const pages=visiblePages();
  if(pv.mode==="scroll"){
    const p=pages[idx];const target=scrollToId||("pvpage_"+(p&&p.name));
    renderPreview();
    setTimeout(()=>{const el=document.getElementById(target);if(el)el.scrollIntoView({behavior:"smooth",block:"start"});},20);
  }else{
    pv.page=idx;document.getElementById("pvBody").scrollTop=0;renderPreview();
    if(scrollToId)setTimeout(()=>{const el=document.getElementById(scrollToId);if(el)el.scrollIntoView({behavior:"smooth",block:"start"});},40);
  }
}

/* ===================== CONTOH ===================== */
const EXAMPLE={specVersion:"1.1",id:"se2026-listing-demo",title:{id:"Listing Usaha SE2026"},version:"1.1.0",locales:["id"],defaultLocale:"id",
  settings:{mode:["capi"],navigation:{mode:"section",showProgress:true,allowBack:true,gateRequired:true}},
  referenceData:{kabupaten:{items:[{code:"6472",label:"Samarinda"},{code:"6471",label:"Balikpapan"}]},kategori_kbli:{items:[{code:"G",label:"Perdagangan"},{code:"I",label:"Akomodasi & Makan Minum"}]}},
  pages:[
    {kind:"page",name:"page_identitas",title:"Identitas Wilayah",components:[
      {kind:"block",name:"blok_wilayah",title:"Wilayah",components:[
        {kind:"section",name:"sec_wilayah",title:"",components:[
          {kind:"field",name:"kabupaten_kota",type:"select",label:{id:"Kabupaten/Kota"},optionsRef:"kabupaten",required:true},
          {kind:"field",name:"titik",type:"geopoint",label:{id:"Titik Lokasi"}}
        ]}
      ]}
    ]},
    {kind:"page",name:"page_usaha",title:"Daftar Usaha",components:[
      {kind:"block",name:"blok_usaha",title:"Usaha pada bangunan",components:[
        {kind:"section",name:"sec_usaha",title:"",components:[
          {kind:"field",name:"ada_usaha",type:"radio",label:{id:"Ada usaha?"},required:true,options:[{value:true,label:{id:"Ya"}},{value:false,label:{id:"Tidak"}}]},
          {kind:"field",name:"jumlah_usaha",type:"integer",label:{id:"Jumlah usaha"},min:0,max:50,required:true,visibleWhen:"${ada_usaha} == true"},
          {kind:"roster",name:"daftar_usaha",title:"Rincian Usaha",rosterType:"separate",countFrom:"jumlah_usaha",itemLabel:{id:"Usaha {{index}}: ${nama_usaha}"},components:[
            {kind:"field",name:"nama_usaha",type:"text",label:{id:"Nama Usaha"},required:true,maxLength:150},
            {kind:"field",name:"kategori",type:"select",label:{id:"Kategori KBLI"},optionsRef:"kategori_kbli",required:true},
            {kind:"field",name:"tenaga_kerja",type:"integer",label:{id:"Tenaga Kerja"},unit:"orang",min:1,required:true,validations:[{test:"${tenaga_kerja} <= 10000",message:{id:"Cek lagi, terlalu besar."},severity:"warning"}]}
          ]}
        ]}
      ]}
    ]}
  ]};

/* ===================== BOOT ===================== */
buildPalette();
view={type:"page",uid:state.pages[0].uid};
render();

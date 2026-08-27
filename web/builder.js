"use strict";
const uid=(()=>{let i=0;return()=>"u"+(++i)+Math.random().toString(36).slice(2,6);})();

/* ---- field type taxonomy ---- */
const TYPES={input:["text","email","textarea","number","integer","decimal","currency","range","rating","calculated","hidden"],
  choice:["select","multiselect","radio","checkbox","boolean"],time:["date","time","datetime"],
  media:["geopoint","photo","file","signature","barcode"],struct:["markdown","note"]};
const CAT_OF={}; Object.entries(TYPES).forEach(([c,a])=>a.forEach(t=>CAT_OF[t]=c));
const CAT_VAR={input:"--input",choice:"--choice",time:"--time",media:"--media",struct:"--struct",node:"--node"};
const LABELS={text:"Short text",email:"Email",textarea:"Long text",number:"Number",integer:"Whole number",decimal:"Decimal",currency:"Currency",range:"Slider",rating:"Rating",calculated:"Calculated",hidden:"Hidden",select:"Dropdown",multiselect:"Multiple choice",radio:"Radio",checkbox:"Checkbox",boolean:"Yes/No",date:"Date",time:"Time",datetime:"Date+time",geopoint:"GPS point",photo:"Photo",file:"File",signature:"Signature",barcode:"Barcode",note:"Note (HTML)",markdown:"Description (Markdown)"};
const CHOICE=new Set(["select","multiselect","radio","checkbox"]);
const NUMERIC=new Set(["number","integer","decimal","currency","range","rating"]);
const TEXTY=new Set(["text","textarea"]);
const DATETIME=new Set(["date","time","datetime"]);
const DT_INPUT_TYPE={date:"date",time:"time",datetime:"datetime-local"};

/* ===================== EXPRESSION ENGINE ===================== */
const Expr=(function(){
  const FUNCS=new Set(["isempty","notempty","len","count","sum","avg","min","max","in","today","age","regex","if","number","round","abs","floor","ceil","contains","upper","lower","trim"]);
  function tokenize(src){
    const t=[];let i=0;const n=src.length;const id=c=>/[A-Za-z0-9_]/.test(c);
    while(i<n){let c=src[i];
      if(c===" "||c==="\t"||c==="\n"||c==="\r"){i++;continue;}
      if(c==="$"&&src[i+1]==="{"){let j=i+2,s="";while(j<n&&src[j]!=="}")s+=src[j++];if(src[j]!=="}")throw new Error("${ } was never closed");i=j+1;t.push({t:"ref",v:s.trim()});continue;}
      if(c==="'"||c==='"'){const q=c;let j=i+1,s="";while(j<n&&src[j]!==q){if(src[j]==="\\"){s+=src[j+1];j+=2;}else s+=src[j++];}if(src[j]!==q)throw new Error("unterminated string");i=j+1;t.push({t:"str",v:s});continue;}
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
      throw new Error("unknown character: "+c);
    }
    return t;
  }
  function parse(src){
    const toks=tokenize(src);let p=0;
    const peek=()=>toks[p],next=()=>toks[p++];
    const expect=k=>{if(!toks[p]||toks[p].t!==k)throw new Error("expected '"+k+"'");return toks[p++];};
    function or(){let l=and();while(peek()&&peek().t==="op"&&peek().v==="||"){next();l={type:"bin",op:"||",l,r:and()};}return l;}
    function and(){let l=eq();while(peek()&&peek().t==="op"&&peek().v==="&&"){next();l={type:"bin",op:"&&",l,r:eq()};}return l;}
    function eq(){let l=cmp();while(peek()&&peek().t==="op"&&(peek().v==="=="||peek().v==="!=")){const o=next().v;l={type:"bin",op:o,l,r:cmp()};}return l;}
    function cmp(){let l=add();while(peek()&&peek().t==="op"&&["<","<=",">",">="].includes(peek().v)){const o=next().v;l={type:"bin",op:o,l,r:add()};}return l;}
    function add(){let l=mul();while(peek()&&peek().t==="op"&&(peek().v==="+"||peek().v==="-")){const o=next().v;l={type:"bin",op:o,l,r:mul()};}return l;}
    function mul(){let l=un();while(peek()&&peek().t==="op"&&["*","/","%"].includes(peek().v)){const o=next().v;l={type:"bin",op:o,l,r:un()};}return l;}
    function un(){if(peek()&&peek().t==="op"&&(peek().v==="!"||peek().v==="-")){const o=next().v;return {type:"un",op:o,e:un()};}return prim();}
    function prim(){const k=peek();if(!k)throw new Error("expression truncated");
      if(k.t==="num"||k.t==="str"||k.t==="bool"){next();return {type:"lit",v:k.v};}
      if(k.t==="null"){next();return {type:"lit",v:null};}
      if(k.t==="ref"){next();return {type:"ref",name:k.v};}
      if(k.t==="lp"){next();const e=or();expect("rp");return e;}
      if(k.t==="lb"){next();const items=[];if(peek()&&peek().t!=="rb"){items.push(or());while(peek()&&peek().t==="comma"){next();items.push(or());}}expect("rb");return {type:"arr",items};}
      if(k.t==="id"){next();
        if(peek()&&peek().t==="lp"){const fn=k.v.toLowerCase();if(!FUNCS.has(fn))throw new Error("unknown function: "+k.v);next();const args=[];if(peek()&&peek().t!=="rp"){args.push(or());while(peek()&&peek().t==="comma"){next();args.push(or());}}expect("rp");return {type:"call",fn,args};}
        throw new Error("use ${...} to reference a field, not '"+k.v+"'");}
      throw new Error("unexpected token");
    }
    const ast=or();if(p<toks.length)throw new Error("unexpected trailing token");return ast;
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

/* ---- markdown renderer (for Description elements) ---- */
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
const ACCEPT={page:["block"],block:["section","roster","field"],section:["section","field","roster"],roster:["block","section","field"]};

let state=blankState(); let selected=null; let selectedSet=new Set(); let view={type:"page",uid:null};
const collapsed={sb1:false,sb2:false};

function blankState(){
  const p={uid:uid(),kind:"page",name:"page_1",title:"Page 1",visibleWhen:"",components:[]};
  return {id:"new-instrument",title:"New Instrument",version:"1.0.0",acronym:"",locales:["id"],defaultLocale:"id",
    settings:{mode:["capi"],navigation:{mode:"section",showProgress:true,allowBack:true,gateRequired:false},offline:{enabled:false}},
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
function newRoster(rt){return {uid:uid(),kind:"roster",name:autoName("roster"),title:"",rowTitle:"",rosterType:rt||"inline",min:"",max:"",countFrom:"",requiredRows:false,itemLabel:"",rowDefaults:"",rowDisplay:[],visibleWhen:"",validations:[],components:[]};}
function newField(type){const f={uid:uid(),kind:"field",type,name:autoName(type),label:"",hint:"",required:false,readOnly:false,promptOnAdd:false,visibleWhen:"",enableWhen:"",requiredWhen:"",allowRemark:false,defaultValue:""};
  if(CHOICE.has(type)){f.options=[{value:"1",label:"Option 1"}];f.optionSource="manual";f.optionsRef="";f.optionsFilterBy="";f.optionsApi={};}
  if(NUMERIC.has(type)){f.min="";f.max="";f.step="";f.unit="";}
  if(TEXTY.has(type)){f.maxLength="";f.pattern="";f.placeholder="";}
  if(type==="photo"){f.autoCompress=true;f.maxPhotoKB="";}
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
    selected=copy.uid;selectedSet=new Set([copy.uid]);view={type:"page",uid:copy.uid};render();return;
  }

  const target=selected?findNode(selected):null;
  // 1) try pasting INSIDE the target if it accepts this kind
  if(target&&kindAccepted(target.kind,clipboard.kind)){
    target.components=target.components||[];target.components.push(copy);
    selected=copy.uid;selectedSet=new Set([copy.uid]);render();return;
  }
  // 2) try pasting as a SIBLING after the target
  if(target){
    const owner=ownerOf(target.uid),ownerKind=owner?owner.kind:"page";
    const arr=parentArrayOf(target.uid);
    if(arr&&kindAccepted(ownerKind,clipboard.kind)){
      const i=arr.indexOf(target);arr.splice(i+1,0,copy);
      selected=copy.uid;selectedSet=new Set([copy.uid]);render();return;
    }
  }
  // 3) fallback: no target yet, but a page is on screen and the clipboard holds a block
  if(!target&&view.type==="page"&&clipboard.kind==="block"){
    const pg=findNode(view.uid);
    if(pg){pg.components=pg.components||[];pg.components.push(copy);selected=copy.uid;selectedSet=new Set([copy.uid]);render();return;}
  }
  alert("There is no valid location for this element. Select a target section/block/page first, then paste.");
}

/* ===================== PALETTE (sidebar 1) ===================== */
function buildPalette(){
  const pal=document.getElementById("palette");
  let html=`<div class="pal-group"><div class="pal-h">Navigation</div>
    ${chip("__block","Block (card)","--block")}
    ${chip("__section","Section (border)","--section")}
    ${chip("__roster_inline","Roster — inline","--roster")}
    ${chip("__roster_separate","Roster — subpage","--roster")}</div>`;
  const groups=[["Input",TYPES.input],["Choices",TYPES.choice],["Date & Time",TYPES.time],["Media & Location",TYPES.media],["Other",TYPES.struct]];
  for(const [t,arr] of groups){html+=`<div class="pal-group"><div class="pal-h">${t}</div>`;arr.forEach(x=>html+=chip(x,LABELS[x],CAT_VAR[CAT_OF[x]]));html+=`</div>`;}
  html+=`<div class="hint">Block → Section → field. A Roster can sit in a Block/Section. A Section can sit inside a Roster. Inline shows on this page; subpages appear in the Pages panel.</div>`;
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
    row.innerHTML=`<span class="pt">${esc(p.title||p.name)}</span><button class="px" title="Delete page">×</button>`;
    row.addEventListener("click",e=>{if(e.target.classList.contains("px"))return;openPage(p.uid);select(p.uid);});
    row.querySelector(".px").addEventListener("click",ev=>{ev.stopPropagation();if(state.pages.length<=1){alert("At least one page is required.");return;}if(confirm("Delete this page?")){removeNode(p.uid);view={type:"page",uid:state.pages[0].uid};selected=null;selectedSet=new Set();render();}});
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

/* ===================== UNDO / REDO ===================== */
/* Snapshots of the whole instrument rather than a log of edits.

   The builder mutates `state` from dozens of places — drag and drop, the inspector,
   paste, bulk actions, the roster editor. Recording an inverse operation at each of
   those sites would mean finding all of them and getting every one right, and any that
   was missed would corrupt the history silently. Comparing serialised snapshots after
   the fact cannot miss a mutation, because every one of those paths ends in render()
   or softUpdate().

   The cost is a JSON round-trip per change. On an instrument the size of SE2026 that
   is well under a millisecond, and it only runs after the debounce settles. */
const History=(function(){
  const LIMIT=80;                 // ~80 steps back; beyond that the oldest is dropped
  const QUIET_MS=400;             // typing a label is one undo step, not one per letter
  let past=[],future=[],current=null,applying=false,timer=null;

  const snap=()=>JSON.stringify(state);

  function commit(){
    timer=null;
    const s=snap();
    if(s===current)return;
    if(current!==null){past.push(current);if(past.length>LIMIT)past.shift();}
    current=s;
    future.length=0;              // a fresh edit abandons the redo branch
    paint();
  }
  // Called from render()/softUpdate(), so it runs after every mutation path.
  function record(){
    if(applying)return;           // replaying a snapshot must not record itself
    clearTimeout(timer);
    timer=setTimeout(commit,QUIET_MS);
  }
  function flush(){if(timer){clearTimeout(timer);commit();}}

  function apply(s){
    applying=true;
    try{
      state=JSON.parse(s);
      // The selection and the open page may name nodes this snapshot never had.
      if(selected&&!findNode(selected)){selected=null;}
      selectedSet=new Set([...selectedSet].filter(uid=>findNode(uid)));
      if(view.type==="page"&&!state.pages.find(p=>p.uid===view.uid))view={type:"page",uid:state.pages[0].uid};
      if(view.type==="roster"&&!findNode(view.uid))view={type:"page",uid:state.pages[0].uid};
      render();
    }finally{applying=false;}
    paint();
  }

  function undo(){
    flush();                      // an edit still inside the debounce window counts
    if(!past.length)return;
    future.push(current);
    current=past.pop();
    apply(current);
  }
  function redo(){
    flush();
    if(!future.length)return;
    past.push(current);
    current=future.pop();
    apply(current);
  }
  function reset(){flush();past=[];future=[];current=snap();paint();}

  function paint(){
    const u=document.getElementById("btnUndo"),r=document.getElementById("btnRedo");
    if(u){u.disabled=!past.length;u.title=past.length?`Undo (${past.length})`:"Nothing to undo";}
    if(r){r.disabled=!future.length;r.title=future.length?`Redo (${future.length})`:"Nothing to redo";}
  }

  return {record,undo,redo,reset,flush,
    get depth(){return {past:past.length,future:future.length};}};
})();

/* ===================== UNSAVED WORK ===================== */
/* Until now the builder held the instrument in memory and nowhere else. Closing the
   tab, pressing Back, or letting the session expire threw the work away with no
   warning and no copy. Undo/redo made that worse rather than better: longer editing
   sessions mean more to lose in one wrong click.

   Three layers, cheapest first:
     1. a dirty marker, so the Save button says whether anything is outstanding
     2. a beforeunload prompt, so leaving is a decision rather than an accident
     3. a copy in localStorage, so even a crash or a killed tab is recoverable

   The baseline is serialize() rather than `state`: that is what actually gets sent to
   the server, and it leaves out the uids, which are regenerated on every load and
   would otherwise make a freshly loaded instrument look modified. */
const Draft=(function(){
  const PREFIX="eform_builder_draft:";
  const SAVE_MS=1500;                    // quiet period before a copy is written
  let key=null,baseline=null,timer=null,ready=false;

  function ser(){try{return JSON.stringify(serialize());}catch(_){return null;}}
  function store(){try{
    const s=ser();if(s===null)return;
    localStorage.setItem(key,JSON.stringify({at:Date.now(),schema:s}));
  }catch(_){/* quota or private mode — the other two layers still apply */}}
  function clearStored(){try{localStorage.removeItem(key);}catch(_){}}

  function isDirty(){if(!ready)return false;const s=ser();return s!==null&&s!==baseline;}

  function paint(){
    const btn=document.getElementById("btnSave");if(!btn)return;
    const dirty=isDirty();
    btn.classList.toggle("dirty",dirty);
    btn.title=dirty?"There are unsaved changes":"Everything is saved";
  }

  // Called from render()/softUpdate(), so it sees every mutation path.
  function touch(){
    if(!ready)return;
    paint();
    clearTimeout(timer);
    timer=setTimeout(()=>{if(isDirty())store();else clearStored();},SAVE_MS);
  }

  /* The instrument on screen now matches the server. An id is passed the first time a
     brand-new instrument is saved: its copy was filed under "new", and that entry has
     to go rather than linger to be offered back on the next blank builder. */
  function markSaved(formId){
    clearTimeout(timer);
    clearStored();
    if(formId)key=PREFIX+formId;
    baseline=ser();
    clearStored();
    paint();
  }

  function offerRecovery(stored){
    const bar=document.createElement("div");
    bar.className="recover-bar";
    const when=new Date(stored.at);
    bar.innerHTML=`<span>An unsaved copy of this instrument was left in this browser on
      <b>${esc(when.toLocaleString())}</b>. It has not been compared with the version on the server.</span>
      <button class="btn" id="recYes">Restore it</button>
      <button class="btn ghost" id="recNo">Discard</button>`;
    document.querySelector(".canvas").prepend(bar);
    bar.querySelector("#recYes").addEventListener("click",()=>{
      try{
        importJSON(JSON.parse(stored.schema));   // resets History for the new content
        bar.remove();
        // Deliberately left dirty: the restored copy does NOT match the server, and
        // saying otherwise would invite the user to close the tab and lose it again.
        paint();
      }catch(e){alert("The stored copy could not be read: "+e.message);}
    });
    bar.querySelector("#recNo").addEventListener("click",()=>{clearStored();bar.remove();});
  }

  /* Called by the bridge once the instrument for this page is in place. */
  function init(formId){
    key=PREFIX+(formId||"new");
    baseline=ser();
    ready=true;
    paint();
    let stored=null;
    try{stored=JSON.parse(localStorage.getItem(key)||"null");}catch(_){}
    // Only worth offering when it differs from what is on screen.
    if(stored&&stored.schema&&stored.schema!==baseline)offerRecovery(stored);
    else if(stored)clearStored();
  }

  window.addEventListener("beforeunload",e=>{
    if(!isDirty())return;
    store();                 // last chance to keep a copy before the tab goes
    e.preventDefault();
    e.returnValue="";        // required for the browser to show its own prompt
  });

  const api={init,markSaved,touch,isDirty};
  // `const` at script scope does not become a window property, and builder-bridge.js
  // reaches for it as window.Draft — so it is published explicitly.
  window.Draft=api;
  return api;
})();

/* ===================== CANVAS ===================== */
function render(){
  if(!state.pages.find(p=>p.uid===view.uid) && view.type==="page") view={type:"page",uid:state.pages[0].uid};
  if(view.type==="roster" && !findNode(view.uid)) view={type:"page",uid:state.pages[0].uid};
  document.getElementById("instTitle").value=state.title||"";
  renderPages(); renderCanvas(); renderInspector(); runValidation(); applyCols();
  History.record(); Draft.touch();
}
function refreshCard(node){const el=document.querySelector(`#stage [data-uid="${node.uid}"]`);if(el)el.replaceWith(renderNode(node));}
function softUpdate(){renderPages();runValidation();const n=selected&&findNode(selected);if(n)refreshCard(n);History.record();Draft.touch();}
function renderCanvas(){
  const head=document.getElementById("cvHead"), stage=document.getElementById("stage"); stage.innerHTML="";
  if(view.type==="roster"){
    const r=findNode(view.uid), pg=pageOf(r.uid);
    head.innerHTML=`<div class="eyebrow">Roster row template</div><button class="btn ghost back" id="backBtn">← ${esc(pg.title||pg.name)}</button>`;
    head.querySelector("#backBtn").addEventListener("click",()=>{openPage(pg.uid);select(r.uid);render();});
    const wrap=document.createElement("div");wrap.className="roster-inline";
    wrap.appendChild(nodeHead(r,"roster",`Roster: ${r.title||r.name}`));
    wrap.appendChild(dropzone(r,["block","section","field"],"Drag a Block, Section, or field — repeated per row"));
    stage.appendChild(wrap); return;
  }
  const page=findNode(view.uid);
  head.innerHTML=`<div class="eyebrow">Page · ${esc(page.title||page.name)}</div>`;
  stage.appendChild(dropzone(page,["block"],"Drag a Block onto this page"));
}

function nodeHead(node,kind,placeholder){
  const h=document.createElement("div"); h.className="node-head";
  h.innerHTML=`<span class="tag ${kind}">${kind}</span><input class="ti" value="${esc(node.title||"")}" placeholder="${esc(placeholder||kind+" title (optional)")}">
    <button class="icon-btn" data-a="sel" title="Settings">⚙</button><button class="icon-btn danger" data-a="del" title="Delete">🗑</button>`;
  h.querySelector(".ti").addEventListener("input",e=>{node.title=e.target.value;runValidation();renderPages();});
  h.querySelector('[data-a="sel"]').addEventListener("click",e=>{e.stopPropagation();select(node.uid,e.ctrlKey||e.metaKey||e.shiftKey);});
  h.querySelector('[data-a="del"]').addEventListener("click",e=>{e.stopPropagation();if(confirm(`Delete this ${kind} and everything inside it?`)){removeNode(node.uid);selected=null;selectedSet=new Set();render();}});
  h.addEventListener("click",e=>{if(!e.target.closest("input,button"))select(node.uid,e.ctrlKey||e.metaKey||e.shiftKey);});
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
    const el=document.createElement("div");el.className="block"+(selectedSet.has(n.uid)?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"block"));
    el.appendChild(dropzone(n,["section","roster","field"],"Drag a Section, Roster, or field into the block"));
    wireDrag(el,n); return el;
  }
  if(n.kind==="section"){
    const el=document.createElement("div");el.className="section"+(selectedSet.has(n.uid)?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"section"));
    el.appendChild(dropzone(n,["section","field","roster"],"Drag a Section, field, or Roster into the section"));
    wireDrag(el,n); return el;
  }
  if(n.kind==="roster"){
    if(n.rosterType==="separate"){
      const el=document.createElement("div");el.className="roster-link"+(selectedSet.has(n.uid)?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
      el.innerHTML=`<span class="ri">⊞</span><div class="rt"><b>${esc(n.title||n.name)}</b><span>Roster subhalaman · ${(n.components||[]).length} field${n.countFrom?" · ×"+esc(n.countFrom):""}</span></div><span class="go">open →</span>`;
      el.addEventListener("click",e=>{if(e.target.closest(".go")||!e.target.closest("button")){ if(e.detail===2){view={type:"roster",uid:n.uid};} select(n.uid,e.ctrlKey||e.metaKey||e.shiftKey);} render();});
      el.querySelector(".go").addEventListener("click",e=>{e.stopPropagation();view={type:"roster",uid:n.uid};select(n.uid);render();});
      wireDrag(el,n); return el;
    }
    const el=document.createElement("div");el.className="roster-inline"+(selectedSet.has(n.uid)?" sel":"");el.dataset.uid=n.uid;el.draggable=true;
    el.appendChild(nodeHead(n,"roster",`Roster inline: ${n.title||n.name}`));
    el.appendChild(dropzone(n,["block","section","field"],"Drag a Block, Section, or field — repeated per row"));
    wireDrag(el,n); return el;
  }
  // field card
  const cat=CAT_OF[n.type];
  const el=document.createElement("div");el.className="card"+(selectedSet.has(n.uid)?" sel":"");el.dataset.uid=n.uid;el.draggable=true;el.style.setProperty("--cat",`var(${CAT_VAR[cat]})`);
  const badges=[`<span class="badge nm">${esc(n.name||"?")}</span>`];
  if(n.skips&&n.skips.length)badges.push(`<span class="badge skip">skip →</span>`);
  if(n.optionSource==="api"||(n.optionsApi&&n.optionsApi.url))badges.push(`<span class="badge">API</span>`);
  else if(n.optionsRef)badges.push(`<span class="badge">ref:${esc(n.optionsRef)}</span>`);
  else if(n.options&&n.options.length){
    const shown=n.options.filter(o=>!o.hidden).length,hidden=n.options.length-shown;
    badges.push(`<span class="badge">${shown} option${shown===1?"":"s"}</span>`);
    if(hidden)badges.push(`<span class="badge">${hidden} hidden</span>`);
  }
  if(n.validations&&n.validations.length)badges.push(`<span class="badge">${n.validations.length} cek</span>`);
  if(n.visibleWhen)badges.push(`<span class="badge">⊘ kondisi</span>`);
  const lbl=(n.type==="note"||n.type==="markdown")?`<span style="color:var(--ink-soft)">${esc(((n.type==="markdown"?(n.markdown||""):String(n.html||"").replace(/<[^>]+>/g," ")).replace(/[#>*`_-]/g," ").trim().slice(0,70))||"(empty label)")}</span>`:(n.label?esc(n.label):`<span class="empty">no label</span>`);
  el.innerHTML=`<div class="crail"></div><div class="body"><div class="top"><span class="ty">${n.type}</span>${n.required?'<span class="req">＊</span>':''}</div><div class="lbl">${lbl}</div><div class="meta">${badges.join("")}</div></div><div class="grip">⋮⋮</div>`;
  el.querySelector(".body").addEventListener("click",e=>select(n.uid,e.ctrlKey||e.metaKey||e.shiftKey));
  wireDrag(el,n); return el;
}

/* ===================== DRAG & DROP ===================== */
const dnd={payload:null}; let placeholder=null;
function clearPlaceholder(){if(placeholder&&placeholder.parentNode)placeholder.parentNode.removeChild(placeholder);placeholder=null;}
function wireDrag(el,n){
  el.addEventListener("dragstart",e=>{
    e.stopPropagation();
    if(selectedSet.has(n.uid)&&selectedSet.size>1){
      dnd.payload={mode:"move-multi",ids:filterTopLevel([...selectedSet]),primaryId:n.uid};
    }else{
      dnd.payload={mode:"move",id:n.uid};
    }
    el.classList.add("dragging");e.dataTransfer.effectAllowed="move";e.dataTransfer.setData("text/plain","move");
  });
  el.addEventListener("dragend",()=>{el.classList.remove("dragging");clearPlaceholder();});
}
function draggedKind(){
  if(!dnd.payload)return null;
  if(dnd.payload.mode==="new"){const t=dnd.payload.type;return t==="__block"?"block":t==="__section"?"section":t.startsWith("__roster")?"roster":"field";}
  if(dnd.payload.mode==="move-multi"){const node=findNode(dnd.payload.primaryId);return node?node.kind:null;}
  const node=findNode(dnd.payload.id);return node?node.kind:null;
}
function wireDropzone(dz,owner,accept){
  dz.addEventListener("dragover",e=>{
    if(!dnd.payload||dnd.payload.mode==="page")return;
    e.preventDefault();e.stopPropagation();
    const k=draggedKind();
    if(!accept.includes(k)){dz.classList.add("reject");dz.classList.remove("over");clearPlaceholder();return;}
    if(dnd.payload.mode==="move"){const dragged=findNode(dnd.payload.id);if(dragged&&nodeContains(dragged,owner.uid))return;}
    if(dnd.payload.mode==="move-multi"){if(dnd.payload.ids.every(id=>{const nd=findNode(id);return nd&&nodeContains(nd,owner.uid);}))return;}
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
    if(dnd.payload.mode==="new"){const node=newFromType(dnd.payload.type);arr.splice(index,0,node);selected=node.uid;selectedSet=new Set([node.uid]);}
    else if(dnd.payload.mode==="move-multi"){
      // Collect the nodes that may legitimately move into this dropzone
      const docOrder=allNodes();
      const toMove=dnd.payload.ids
        .map(id=>({node:findNode(id),src:parentArrayOf(id)}))
        .filter(({node,src})=>node&&src&&accept.includes(node.kind)&&!nodeContains(node,owner.uid))
        .sort((a,b)=>docOrder.indexOf(a.node)-docOrder.indexOf(b.node));
      if(toMove.length>0){
        let ins=index;
        // Remove from the source (descending per array so indices do not shift)
        const bySrc=new Map();
        toMove.forEach(item=>{if(!bySrc.has(item.src))bySrc.set(item.src,[]);bySrc.get(item.src).push(item);});
        bySrc.forEach((items,srcArr)=>{
          items.sort((a,b)=>srcArr.indexOf(b.node)-srcArr.indexOf(a.node));
          items.forEach(({node})=>{const i=srcArr.indexOf(node);if(i>=0){srcArr.splice(i,1);if(srcArr===arr&&i<ins)ins--;}});
        });
        // Insert at the destination in original document order
        toMove.forEach(({node},off)=>arr.splice(ins+off,0,node));
      }
    }
    else{const id=dnd.payload.id,dragged=findNode(id),src=parentArrayOf(id);
      if(dragged&&src&&!nodeContains(dragged,owner.uid)){const from=src.indexOf(dragged);src.splice(from,1);let ins=index;if(src===arr&&from<index)ins--;arr.splice(ins,0,dragged);}}
    dnd.payload=null;render();
  });
}
function childAfter(dz,y){const items=[...dz.children].filter(c=>c!==placeholder&&(c.classList.contains("card")||c.classList.contains("block")||c.classList.contains("section")||c.classList.contains("roster-inline")||c.classList.contains("roster-link")));for(const c of items){const r=c.getBoundingClientRect();if(y<r.top+r.height/2)return c;}return null;}
function phIndex(dz,arr){const kids=[...dz.children].filter(c=>c===placeholder||c.dataset&&c.dataset.uid);const i=kids.indexOf(placeholder);return i<0?arr.length:i;}
function nodeContains(node,targetUid){if(node.uid===targetUid)return true;return (node.components||[]).some(c=>nodeContains(c,targetUid));}
// From a set of uids, drop any that are descendants of another uid in the same set
function filterTopLevel(ids){const s=new Set(ids);return ids.filter(id=>{const n=findNode(id);if(!n)return false;for(const oid of s){if(oid===id)continue;const on=findNode(oid);if(on&&nodeContains(on,id))return false;}return true;});}
function duplicateSelected(){
  const topIds=filterTopLevel([...selectedSet]);
  const docOrder=allNodes();
  const items=topIds
    .map(id=>({node:findNode(id),arr:parentArrayOf(id)}))
    .filter(x=>x.node&&x.arr)
    .sort((a,b)=>docOrder.indexOf(a.node)-docOrder.indexOf(b.node));
  if(!items.length)return;
  const newSet=new Set();
  // Process descending per array so indices do not shift
  const byArr=new Map();
  items.forEach(item=>{if(!byArr.has(item.arr))byArr.set(item.arr,[]);byArr.get(item.arr).push(item);});
  byArr.forEach((its,arr)=>{
    its.sort((a,b)=>arr.indexOf(b.node)-arr.indexOf(a.node));
    its.forEach(({node})=>{
      const copy=JSON.parse(JSON.stringify(node));reuid(copy);copy.name=uniqueCopyName(node.name);
      const i=arr.indexOf(node);arr.splice(i+1,0,copy);newSet.add(copy.uid);
    });
  });
  selected=[...newSet].at(-1);selectedSet=newSet;render();
}

/* ===================== SELECTION & INSPECTOR ===================== */
function select(id,multi=false){
  if(multi&&id){
    if(selectedSet.has(id)){selectedSet.delete(id);selected=selectedSet.size>0?[...selectedSet].at(-1):null;}
    else{selectedSet.add(id);selected=id;}
  }else{
    selectedSet=new Set(id?[id]:[]);selected=id;
    const n=findNode(id);if(n&&view.type==="page"){const pg=pageOf(id);if(pg&&pg.uid!==view.uid)view={type:"page",uid:pg.uid};}
  }
  switchTab("props");render();
}
function renderInspector(){
  const pane=document.getElementById("paneProps");
  if(selectedSet.size>1){
    // Bulk property edits only make sense for fields; a selection may also hold pages,
    // blocks, sections and rosters, and those are simply left out of the count.
    const picked=[...selectedSet].map(findNode).filter(Boolean);
    const fields=picked.filter(n=>n.kind==="field");
    const BULK=[["required","Required"],["readOnly","Read-only"],["allowRemark","Allow remarks"]];
    const bulkRows=fields.length?BULK.map(([k,label])=>{
      const on=fields.filter(f=>!!f[k]).length;
      const stateTxt=on===0?"none":(on===fields.length?"all":`${on} of ${fields.length}`);
      return `<div class="bulk-row">
        <span class="bulk-lab">${label}</span>
        <span class="bulk-state">${stateTxt}</span>
        <button class="btn ghost bulk-b" data-bulk="${k}" data-on="1"${on===fields.length?" disabled":""}>On</button>
        <button class="btn ghost bulk-b" data-bulk="${k}" data-on="0"${on===0?" disabled":""}>Off</button>
      </div>`;
    }).join(""):`<div class="help" style="margin-left:0">No fields in this selection.</div>`;

    pane.innerHTML=`<div style="padding:16px 12px">
      <div style="font-weight:600;font-size:13px;margin-bottom:8px">${selectedSet.size} items selected${fields.length&&fields.length!==picked.length?` · ${fields.length} field${fields.length>1?"s":""}`:""}</div>
      <div class="help" style="margin-left:0;margin-bottom:14px">
        <b>Ctrl/Shift+click</b> to add or remove items.<br>
        <b>Drag</b> any one of them to move them all.<br>
        <b>Delete</b> to remove them all.
      </div>
      <div class="gh" style="margin-bottom:6px">Set on all selected fields</div>
      <div style="margin-bottom:14px">${bulkRows}</div>
      <button class="btn" id="dupAllBtn" style="width:100%;margin-bottom:8px">⧉ Duplicate all (${selectedSet.size})</button>
      <button class="btn danger" id="delAllBtn" style="width:100%;margin-bottom:8px">🗑 Delete all (${selectedSet.size})</button>
      <button class="btn ghost" id="clearSelBtn" style="width:100%">Cancel selection</button>
    </div>`;
    pane.querySelectorAll(".bulk-b").forEach(b=>b.addEventListener("click",()=>{
      const k=b.dataset.bulk,on=b.dataset.on==="1";
      fields.forEach(f=>{if(on)f[k]=true;else delete f[k];});
      render();          // re-render so the counts and the cards both catch up
      // Committed straight away rather than left to the debounce. Coalescing is right
      // for typing, but two deliberate clicks are two things the user will expect to
      // undo separately.
      History.flush();
    }));
    pane.querySelector("#dupAllBtn").addEventListener("click",()=>duplicateSelected());
    pane.querySelector("#delAllBtn").addEventListener("click",()=>{if(confirm(`Delete the ${selectedSet.size} selected items?`)){[...selectedSet].forEach(uid=>removeNode(uid));selected=null;selectedSet=new Set();render();}});
    pane.querySelector("#clearSelBtn").addEventListener("click",()=>{selected=null;selectedSet=new Set();render();});
    return;
  }
  const n=selected?findNode(selected):null;
  if(!n){pane.innerHTML=instrumentForm();wireInstrument(pane);return;}
  if(n.kind==="page")pane.innerHTML=navForm(n,"page");
  else if(n.kind==="block")pane.innerHTML=navForm(n,"block");
  else if(n.kind==="section")pane.innerHTML=navForm(n,"section");
  else if(n.kind==="roster")pane.innerHTML=rosterForm(n);
  else pane.innerHTML=fieldForm(n);
  wireForm(pane,n);
}
function instrumentForm(){const nv=state.settings.navigation;const off=state.settings.offline||(state.settings.offline={enabled:false});return `<div class="empty-state"><div class="big">{ }</div>Nothing selected — instrument settings.</div>
  <div class="field"><label>Instrument ID</label><input class="ctrl mono" data-i="id" value="${esc(state.id)}"></div>
  <div class="row2"><div class="field"><label>Version</label><input class="ctrl mono" data-i="version" value="${esc(state.version)}"></div><div class="field"><label>Acronym</label><input class="ctrl" data-i="acronym" value="${esc(state.acronym||"")}"></div></div>
  <div class="row2"><div class="field"><label>Locales</label><input class="ctrl mono" data-i="locales" value="${esc(state.locales.join(","))}"></div><div class="field"><label>Default locale</label><input class="ctrl mono" data-i="defaultLocale" value="${esc(state.defaultLocale)}"></div></div>
  <div class="group"><div class="gh">Navigation</div>
    <div class="field"><label>Mode</label><select class="ctrl" data-i="nav.mode">${opt("scroll","Scroll",nv.mode)}${opt("section","Sections per page",nv.mode)}${opt("field","Fields per screen",nv.mode)}</select></div>
    <label class="check"><input type="checkbox" data-i="nav.gateRequired" ${nv.gateRequired?"checked":""}> Must be completed before continuing</label></div>
  <div class="group"><div class="gh">Offline Mode (PWA)</div>
    <label class="check"><input type="checkbox" data-i="offline.enabled" ${off.enabled?"checked":""}> Enable offline mode</label>
    <div class="help" style="margin-left:0;margin-top:6px">The form can be installed like a native app on a phone and filled in offline — responses are stored on the device and sent automatically once back online. <b>Only applies</b> to share links set as <b>multi-response</b>.</div></div>
  <div class="group"><div class="gh">Lookup source / Reference data (JSON)</div><textarea class="ctrl mono" data-i="referenceData" rows="6" placeholder='{ "kabupaten": { "items":[ {"code":"6472","label":"Samarinda"} ] } }'>${esc(jsonOrEmpty(state.referenceData))}</textarea><div class="help" style="margin-left:0;margin-top:6px">Each table lists its options inline under <code>items</code>, every entry a <code>code</code> and a <code>label</code>. Add <code>parent</code> to an entry to make the table cascade from another field. Reference a table from a field using <b>optionsRef</b>. To pull options from an external service instead, set the field's choice source to <b>API</b>.</div></div>`;}
function wireInstrument(pane){pane.querySelectorAll("[data-i]").forEach(inp=>inp.addEventListener("input",()=>{const k=inp.dataset.i,v=inp.type==="checkbox"?inp.checked:inp.value;if(k.startsWith("nav."))state.settings.navigation[k.slice(4)]=v;else if(k.startsWith("offline."))state.settings.offline[k.slice(8)]=v;else if(k==="locales")state.locales=v.split(",").map(s=>s.trim()).filter(Boolean);else if(k==="referenceData"){try{state.referenceData=v.trim()?JSON.parse(v):{};inp.style.borderColor="";}catch(_){inp.style.borderColor="var(--bad)";}}else state[k]=v;runValidation();}));}

function navForm(n,kind){
  const titleLabel=kind==="page"?"Page title":kind==="block"?"Block title (optional)":"Section title (optional)";
  // Pages & sections: the dataKey is generated automatically and is already unique, but can still be changed if needed.
  const nameField=(kind==="page"||kind==="section")
    ? `<div class="field"><label>Name (dataKey) <span class="help">automatic & unique, can be changed</span></label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>`
    : `<div class="field"><label>Name (dataKey) <span class="help">unique</span></label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>`;
  return `${headBar(kind,n.name)}
  ${nameField}
  <div class="field"><label>${titleLabel}</label><input class="ctrl" data-k="title" value="${esc(n.title||"")}"></div>
  <div class="field"><label>Visible when (visibleWhen)</label><textarea class="ctrl" data-k="visibleWhen" placeholder="\${field} == value">${esc(n.visibleWhen||"")}</textarea></div>`;
}
function rosterForm(n){
  const childFields=(n.components||[]).filter(c=>c.kind==="field");
  const minRows=Math.max(0,Math.floor(Number(n.min)||0));
  const rowDefaultLines=String(n.rowDefaults||"").split(/\r?\n/);
  const dispBlock = `<div class="group"><div class="gh">Fields shown in the row list</div>${childFields.length?childFields.map(f=>`<label class="check"><input type="checkbox" data-rowdisp="${esc(f.name)}" ${(n.rowDisplay||[]).includes(f.name)?"checked":""}> ${esc(f.label||f.name)}</label>`).join(""):`<div class="help" style="margin-left:0">Add a field to the roster first.</div>`}<div class="help" style="margin-left:0;margin-top:6px">For subpage rosters: this field's value becomes each row's summary on the main page.</div></div>`;
  const rowDefaultEditor=(n.rosterType==="separate")
    ? (minRows>0
      ? `<div class="group"><div class="gh">Default row value (auto-fills the first field)</div>${Array.from({length:minRows},(_,i)=>`<div class="field"><label>Row ${i+1}</label><input class="ctrl" data-rowdefault-index="${i}" value="${esc((rowDefaultLines[i]||"").trim())}" placeholder="Example: Business ${i+1}"></div>`).join("")}<div class="help" style="margin-left:0">This value is prefilled into the first field of every row created by Min rows. It never overwrites a value you changed by hand.</div></div>`
      : `<div class="group"><div class="gh">Default row value</div><div class="help" style="margin-left:0;margin-bottom:6px">Set Min rows first so the per-row editor appears. You can also fill it quickly using one value per line.</div><textarea class="ctrl" data-k="rowDefaults" rows="4" placeholder="Usaha Budi&#10;Usaha Rudi&#10;Usaha Dudi">${esc(n.rowDefaults||"")}</textarea></div>`)
    : "";
  return `${headBar("roster",n.name)}
  <div class="field"><label>Name (dataKey)</label><input class="ctrl mono" data-k="name" value="${esc(n.name)}"></div>
  <div class="field"><label>Roster type</label>
    <div class="seg" id="rtSeg"><button data-rt="inline" class="${n.rosterType==="inline"?"on":""}">Inline</button><button data-rt="separate" class="${n.rosterType==="separate"?"on":""}">Subpage</button></div>
    <div class="help" style="margin-left:0;margin-top:6px">${n.rosterType==="inline"?"Input on the same page.":"List rows on the main page; fill each row on a separate page."}</div></div>
  <div class="field"><label>Roster title (optional)</label><input class="ctrl" data-k="title" value="${esc(n.title||"")}"></div>
  <div class="field"><label>Roster row title <span class="help">e.g. "Business" — used in the add-row button &amp; popup</span></label><input class="ctrl" data-k="rowTitle" placeholder="Usaha" value="${esc(n.rowTitle||"")}"></div>
  <div class="row2"><div class="field"><label>Min rows</label><input class="ctrl" type="number" step="1" min="0" inputmode="numeric" data-k="min" value="${esc(n.min??"")}"></div><div class="field"><label>Max rows</label><input class="ctrl" type="number" step="1" min="0" inputmode="numeric" data-k="max" value="${esc(n.max??"")}"></div></div>
  <div class="field"><label>Row count from field (countFrom) <span class="help">automatically generates rows; leave empty to use the "+ Add ${n.rowTitle?esc(n.rowTitle):"row"}" button with a popup</span></label><input class="ctrl mono" data-k="countFrom" value="${esc(n.countFrom||"")}"></div>
  <div class="field"><label class="chk"><input type="checkbox" data-k="requiredRows" ${n.requiredRows?"checked":""}> At least one row must be added (minimum 1 row)</label></div>
  <div class="field"><label>Per-row label (itemLabel)</label><input class="ctrl" data-k="itemLabel" placeholder="Item {{index}}: \${nama}" value="${esc(n.itemLabel||"")}"></div>
  ${rowDefaultEditor}
  ${dispBlock}
  <div class="field"><label>Visible when</label><textarea class="ctrl" data-k="visibleWhen">${esc(n.visibleWhen||"")}</textarea></div>
  ${validationsBlock(n,`Rules for the roster as a whole. <code>\${${esc(n.name)}}</code> is the list of its rows, so <code>len(\${${esc(n.name)}})</code> is how many there are — compare it with another answer, e.g. <code>len(\${${esc(n.name)}}) == \${jml_art}</code>.`)}
  ${n.rosterType==="separate"?`<button class="add-row" id="openRoster">Open the roster template editor →</button>`:""}`;
}
function fieldForm(c){const t=c.type;let html=headBar(t,c.name);
  html+=`<div class="field"><label>Name (dataKey) <span class="help">unique, output column</span></label><input class="ctrl mono" data-k="name" value="${esc(c.name)}"></div>`;
  if(t!=="note")html+=`<div class="field"><label>Question label</label><input class="ctrl" data-k="label" value="${esc(c.label||"")}"></div>`;
  html+=`<div class="field"><label>Hint</label><input class="ctrl" data-k="hint" value="${esc(c.hint||"")}"></div>`;
  if(t==="note")html+=`<div class="field"><label>HTML content</label><textarea class="ctrl" data-k="html" rows="3">${esc(c.html||"")}</textarea></div>`;
  if(t==="markdown")html+=`<div class="field"><label>Description (Markdown)</label><textarea class="ctrl" data-k="markdown" rows="6" placeholder="# Filling instructions&#10;&#10;Answer according to **actual conditions**. See:&#10;- first point&#10;- second point&#10;&#10;> Important note.">${esc(c.markdown||"")}</textarea><div class="help" style="margin-left:0;margin-top:4px">Supports: # heading, **bold**, *italic*, \`kode\`, list (- / 1.), &gt; quote, [text](url), --- rule.</div></div>`;
  if(t==="calculated")html+=`<div class="field"><label>Formula (calculate)</label><textarea class="ctrl" data-k="calculate" placeholder="\${a}+\${b}">${esc(c.calculate||"")}</textarea></div><label class="check" style="margin-top:6px"><input type="checkbox" data-k="autofill" ${c.autofill?"checked":""}> Autofill — filled automatically but can be edited</label>`;
  if(NUMERIC.has(t))html+=`<div class="row3">${mini("min","Min",c.min)}${mini("max","Max",c.max)}${mini("step","Step",c.step)}</div><div class="field"><label>Unit</label><input class="ctrl" data-k="unit" value="${esc(c.unit||"")}"></div>`;
  if(DATETIME.has(t)){const it=DT_INPUT_TYPE[t];html+=`<div class="row2">${mini("min","From",c.min,it)}${mini("max","To",c.max,it)}</div>`;}
  if(TEXTY.has(t))html+=`<div class="row2">${mini("maxLength","Max characters",c.maxLength,"number")}<div class="field"><label>Placeholder</label><input class="ctrl" data-k="placeholder" value="${esc(c.placeholder||"")}"></div></div><div class="field"><label>Pattern (regex)</label><input class="ctrl mono" data-k="pattern" value="${esc(c.pattern||"")}"></div>`;
  if(t==="photo"){
    // Absent means on, so fields built before this option existed keep compressing.
    const ac=c.autoCompress!==false;
    html+=`<div class="group"><div class="gh">Photo upload</div><label class="check"><input type="checkbox" data-k="autoCompress" ${ac?"checked":""}> Compress the photo before uploading</label>${ac?`<div class="field" style="margin-top:8px"><label>Max photo size (KB)</label><input class="ctrl" type="number" step="1" min="1" inputmode="numeric" data-k="maxPhotoKB" placeholder="200" value="${esc(c.maxPhotoKB??"")}"><div class="help" style="margin-left:0;margin-top:4px">Leave empty for the 200 KB default. A very detailed photo may still land slightly above the limit — it is uploaded anyway rather than blocking the enumerator.</div></div>`:`<div class="help" style="margin-left:0;margin-top:6px">The original camera file is uploaded as it is — often several MB per photo.</div>`}</div>`;
  }
  if(CHOICE.has(t))html+=optionsBlock(c);
  const dvHtml=(()=>{
    if(t==="note"||t==="markdown"||t==="calculated")return"";
    if(t==="boolean")return`<div class="field" style="margin-top:8px"><label>Default value</label><select class="ctrl" data-k="defaultValue"><option value="">— none —</option><option value="true"${c.defaultValue==="true"?" selected":""}>Yes</option><option value="false"${c.defaultValue==="false"?" selected":""}>No</option></select></div>`;
    if(CHOICE.has(t)&&(c.optionSource==="manual"||(!c.optionSource&&c.options&&c.options.length))){
      const opts=(c.options||[]).map(o=>{const v=String(o.value??"");const lbl=typeof o.label==="object"?(o.label[state.defaultLocale]||o.label.id||v):(o.label||v);return`<option value="${esc(v)}"${String(c.defaultValue??"")=== v?" selected":""}>${esc(lbl)}</option>`;}).join("");
      return`<div class="field" style="margin-top:8px"><label>Default value</label><select class="ctrl" data-k="defaultValue"><option value="">— none —</option>${opts}</select></div>`;
    }
    return`<div class="field" style="margin-top:8px"><label>Default value</label><input class="ctrl" data-k="defaultValue" value="${esc(c.defaultValue||"")}"></div>`;
  })();
  html+=`<div class="group"><div class="gh">Behavior</div><label class="check"><input type="checkbox" data-k="required" ${c.required?"checked":""}> Required</label><label class="check"><input type="checkbox" data-k="readOnly" ${c.readOnly?"checked":""}> Read-only</label><label class="check"><input type="checkbox" data-k="allowRemark" ${c.allowRemark?"checked":""}> Allow remarks</label><label class="check"><input type="checkbox" data-k="promptOnAdd" ${c.promptOnAdd?"checked":""}> Prompted when adding a row <span class="help">the value can be referenced in labels with <code>{{${c.name}}}</code></span></label>${dvHtml}</div>`;
  html+=`<div class="group"><div class="gh">Conditions & flow</div>${cond("visibleWhen","Visible when",c.visibleWhen)}${cond("enableWhen","Enabled when",c.enableWhen)}${cond("requiredWhen","Required when",c.requiredWhen)}${skipsBlock(c)}</div>`;
  html+=validationsBlock(c); return html;
}
function headBar(kind,name){const cat=CAT_OF[kind]||"node";const colorVar=({page:"--page",block:"--block",section:"--section",roster:"--roster"})[kind]||CAT_VAR[cat];const pasteBtn=clipboard?`<button class="icon-btn" id="pasteBtn" title="Paste the copied ${esc(clipboard.kind)}">📥</button>`:"";return `<div style="display:flex;align-items:center;gap:9px;margin-bottom:14px"><span style="width:4px;height:30px;border-radius:2px;background:var(${colorVar})"></span><div><div style="font-family:var(--mono);font-size:11px;color:var(${colorVar});font-weight:700;text-transform:uppercase">${kind}</div><div style="font-size:11px;color:var(--muted)">${esc(name)}</div></div><button class="icon-btn" id="copyBtn" title="Copy" style="margin-left:auto">📋</button><button class="icon-btn" id="dupBtn" title="Duplicate">⧉</button>${pasteBtn}<button class="icon-btn danger" id="delBtn" title="Delete">🗑</button></div>`;}
function mini(k,l,v,type){const attrs=type==="number"?'type="number" step="1" min="0" inputmode="numeric"':(type?`type="${esc(type)}"`:'');return `<div class="field"><label>${l}</label><input class="ctrl" ${attrs} data-k="${k}" value="${esc(v??"")}"></div>`;}
function cond(k,l,v){return `<div class="field"><label>${l}</label><textarea class="ctrl" data-k="${k}" placeholder="\${field} == value">${esc(v||"")}</textarea></div>`;}
function optionsBlock(c){
  const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
  const seg=`<div class="seg" id="osSeg"><button data-os="manual" class="${mode==="manual"?"on":""}">Manual</button><button data-os="ref" class="${mode==="ref"?"on":""}">Inline</button><button data-os="api" class="${mode==="api"?"on":""}">API</button></div>`;
  let body="";
  if(mode==="manual"){
    /* Hidden options stay in the instrument but are not offered while filling it in.
       Retiring a choice mid-collection is the point: deleting it would leave answers
       already recorded against it pointing at a value the instrument no longer
       describes, and the responses list could no longer put a label on them. */
    const rows=(c.options||[]).map((o,i)=>`<div class="mini${o.hidden?" opt-hidden":""}" data-oi="${i}"><div class="mr"><input class="ctrl" data-of="value" placeholder="value" value="${esc(o.value??"")}"><input class="ctrl" data-of="label" placeholder="label" value="${esc(typeof o.label==="object"?(o.label.id||""):(o.label||""))}"><button class="x" data-orm>×</button></div><input class="ctrl mono" data-of="skipTo" placeholder="skipTo (optional)" value="${esc(o.skipTo||"")}" style="margin-top:6px"><label class="check opt-hide-row"><input type="checkbox" data-of="hidden" ${o.hidden?"checked":""}> Hidden — kept in the data, not offered when filling in</label></div>`).join("");
    body=`<div id="optRows">${rows}</div><button class="add-row" id="addOpt">+ Add option</button>`;
  } else if(mode==="ref"){
    // Only tables that can actually yield options. A table with no items — a leftover
    // "source":"api" one, or simply a malformed entry — would be selectable and then
    // silently produce an empty dropdown; lint() reports those separately.
    const tables=Object.entries(state.referenceData||{}).filter(([,v])=>v&&Array.isArray(v.items)&&v.items.length).map(([k])=>k);
    body = tables.length
      ? `<div class="field"><label>Source table (field)</label><select class="ctrl" data-k="optionsRef"><option value="">— select a table —</option>${tables.map(k=>`<option value="${esc(k)}"${c.optionsRef===k?" selected":""}>${esc(k)}</option>`).join("")}</select></div>`
      : `<div class="help" style="margin-left:0">No inline tables yet. Define one in instrument settings → Reference data first, then pick it here.</div>`;
    body+=`<div class="field"><label>Cascading filter (parent field)</label><input class="ctrl mono" data-k="optionsFilterBy" placeholder="parent field name (optional)" value="${esc(c.optionsFilterBy||"")}"></div>`;
  } else {
    const a=c.optionsApi||{};
    body=`<div class="field"><label>API URL <span class="help">use {dataKey} to substitute the field's value</span></label><input class="ctrl mono" data-api="url" placeholder="https://api.../wilayah?kab={kabupaten_kota}" value="${esc(a.url||"")}"></div>
      <div class="field"><label>Trigger dataKey <span class="help">dataKey that triggers a refetch &amp; must be filled first — comma separated</span></label><input class="ctrl mono" data-api="depKeys" placeholder="provinsi, kabupaten_kota" value="${esc(a.depKeys||"")}"></div>
      <div class="row2"><div class="field"><label>Value field</label><input class="ctrl mono" data-api="valueField" placeholder="kode" value="${esc(a.valueField||"")}"></div><div class="field"><label>Label field</label><input class="ctrl mono" data-api="labelField" placeholder="nama" value="${esc(a.labelField||"")}"></div></div>
      <div class="row2"><div class="field"><label>Parent param <span class="help">cascading</span></label><input class="ctrl mono" data-api="parentParam" placeholder="prov (optional)" value="${esc(a.parentParam||"")}"></div><div class="field"><label>Response path <span class="help">optional</span></label><input class="ctrl mono" data-api="path" placeholder="data" value="${esc(a.path||"")}"></div></div>
      <div class="field"><label>Cascading filter (parent field)</label><input class="ctrl mono" data-k="optionsFilterBy" placeholder="parent field name (optional)" value="${esc(c.optionsFilterBy||"")}"></div>
      <div class="help" style="margin-left:0"><code>{dataKey}</code> in the URL is replaced by that field's value. A trigger dataKey blocks the fetch &amp; resets the choice while it is empty. <code>path</code> if the array is nested.</div>`;
  }
  return `<div class="group"><div class="gh">Choices · source</div>${seg}${body}</div>`;
}
function skipsBlock(c){let rows=(c.skips||[]).map((s,i)=>`<div class="mini" data-si="${i}"><input class="ctrl mono" data-sf="when" placeholder="when (expression)" value="${esc(s.when||"")}"><div class="mr" style="grid-template-columns:1fr auto;margin-top:6px"><input class="ctrl mono" data-sf="to" placeholder="jump to / __end" value="${esc(s.to||"")}"><button class="x" data-srm>×</button></div></div>`).join("");return `<div style="margin-top:6px"><div class="gh" style="margin-bottom:6px">Skips</div><div id="skipRows">${rows}</div><button class="add-row" id="addSkip">+ Add skip</button></div>`;}
function validationsBlock(c,help){let rows=(c.validations||[]).map((v,i)=>`<div class="mini" data-vi="${i}"><input class="ctrl mono" data-vf="test" placeholder="test (TRUE=pass)" value="${esc(v.test||"")}"><div class="mr" style="grid-template-columns:1fr auto;margin-top:6px"><input class="ctrl" data-vf="message" placeholder="message" value="${esc(typeof v.message==="object"?(v.message.id||""):(v.message||""))}"><button class="x" data-vrm>×</button></div><select class="ctrl" data-vf="severity" style="margin-top:6px">${opt("error","error — blocks",v.severity||"error")}${opt("warning","warning — can continue",v.severity||"error")}</select></div>`).join("");return `<div class="group"><div class="gh">Validation</div>${help?`<div class="help" style="margin-left:0;margin-bottom:8px">${help}</div>`:""}<div id="valRows">${rows}</div><button class="add-row" id="addVal">+ Add rule</button></div>`;}

function wireForm(pane,node){
  pane.querySelectorAll("[data-k]").forEach(inp=>{const h=()=>{node[inp.dataset.k]=inp.type==="checkbox"?inp.checked:inp.value;if(node.kind==="roster"&&inp.dataset.k==="min"){render();return;}if(inp.dataset.k==="autoCompress"){render();return;}softUpdate();};inp.addEventListener("input",h);inp.addEventListener("change",h);});
  pane.querySelectorAll("[data-rowdefault-index]").forEach(inp=>{
    const onChange=()=>{
      const idx=Number(inp.getAttribute("data-rowdefault-index"));
      const lines=String(node.rowDefaults||"").split(/\r?\n/);
      while(lines.length<=idx) lines.push("");
      lines[idx]=inp.value;
      while(lines.length && String(lines[lines.length-1]||"").trim()==="") lines.pop();
      node.rowDefaults=lines.join("\n");
      softUpdate();
    };
    inp.addEventListener("input",onChange);
    inp.addEventListener("change",onChange);
  });
  pane.querySelector("#delBtn")?.addEventListener("click",()=>{if(confirm("Delete this?")){removeNode(node.uid);selected=null;selectedSet=new Set();render();}});
  pane.querySelector("#dupBtn")?.addEventListener("click",()=>{const arr=parentArrayOf(node.uid),i=arr.indexOf(node),copy=JSON.parse(JSON.stringify(node));reuid(copy);copy.name=uniqueCopyName(node.name);arr.splice(i+1,0,copy);selected=copy.uid;selectedSet=new Set([copy.uid]);render();});
  pane.querySelector("#copyBtn")?.addEventListener("click",()=>{copyNode(node);render();});
  pane.querySelector("#pasteBtn")?.addEventListener("click",()=>{pasteNode();});
  pane.querySelectorAll("#rtSeg button").forEach(b=>b.addEventListener("click",()=>{node.rosterType=b.dataset.rt;render();}));
  pane.querySelectorAll("[data-rowdisp]").forEach(cb=>cb.addEventListener("change",()=>{node.rowDisplay=node.rowDisplay||[];const nm=cb.getAttribute("data-rowdisp");if(cb.checked){if(!node.rowDisplay.includes(nm))node.rowDisplay.push(nm);}else node.rowDisplay=node.rowDisplay.filter(x=>x!==nm);softUpdate();}));
  pane.querySelector("#openRoster")?.addEventListener("click",()=>{view={type:"roster",uid:node.uid};render();});
  // options
  pane.querySelectorAll("#osSeg button").forEach(b=>b.addEventListener("click",()=>{node.optionSource=b.dataset.os;if(node.optionSource==="api"&&!node.optionsApi)node.optionsApi={url:"",valueField:"",labelField:""};render();}));
  pane.querySelectorAll("[data-api]").forEach(inp=>inp.addEventListener("input",()=>{node.optionsApi=node.optionsApi||{};node.optionsApi[inp.dataset.api]=inp.value;softUpdate();}));
  pane.querySelectorAll("#optRows .mini").forEach(row=>{const i=+row.dataset.oi;row.querySelectorAll("[data-of]").forEach(inp=>{
    const h=()=>{
      const f=inp.dataset.of;
      if(inp.type==="checkbox"){
        // "hidden" is the only boolean here; dropping it when false keeps the exported
        // instrument free of a flag on every single option.
        if(inp.checked)node.options[i][f]=true; else delete node.options[i][f];
        render();   // the row is greyed out, so the whole pane redraws
        return;
      }
      node.options[i][f]=inp.value;softUpdate();
    };
    inp.addEventListener("input",h);inp.addEventListener("change",h);
  });row.querySelector("[data-orm]")?.addEventListener("click",()=>{node.options.splice(i,1);render();});});
  pane.querySelector("#addOpt")?.addEventListener("click",()=>{node.options=node.options||[];node.options.push({value:String(node.options.length+1),label:"Option "+(node.options.length+1)});render();});
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
  if(n.kind==="roster"){const o={kind:"roster",name:n.name,rosterType:n.rosterType};if(clean(n.title))o.title=loc(n.title);if(clean(n.rowTitle))o.rowTitle=n.rowTitle;["min","max"].forEach(k=>{if(clean(n[k]))o[k]=num(n[k]);});if(clean(n.countFrom))o.countFrom=n.countFrom;if(n.requiredRows)o.requiredRows=true;if(clean(n.itemLabel))o.itemLabel=loc(n.itemLabel);if(clean(n.rowDefaults))o.rowDefaults=loc(n.rowDefaults);if(n.rowDisplay&&n.rowDisplay.length)o.rowDisplay=n.rowDisplay;if(clean(n.visibleWhen))o.visibleWhen=n.visibleWhen;if(n.validations&&n.validations.length){const v=n.validations.filter(x=>clean(x.test));if(v.length)o.validations=v.map(x=>({test:x.test,message:loc(x.message||""),severity:x.severity||"error"}));}o.components=n.components.map(serNode);return o;}
  const c=n,o={kind:"field",name:c.name,type:c.type};
  if(c.type!=="note"&&clean(c.label))o.label=loc(c.label);
  if(clean(c.hint))o.hint=loc(c.hint);
  if(c.type==="note"&&clean(c.html))o.html=loc(c.html);
  if(c.type==="markdown"&&clean(c.markdown))o.markdown=loc(c.markdown);
  if(c.type==="calculated"&&clean(c.calculate))o.calculate=c.calculate;if(c.type==="calculated"&&c.autofill)o.autofill=true;
  if(clean(c.unit))o.unit=c.unit;
  ["min","max","step","maxLength"].forEach(k=>{if(clean(c[k]))o[k]=num(c[k]);});
  // Only the off state and an explicit budget are written; an untouched field stays
  // absent from the schema and falls back to the runtime default, which is what keeps
  // the photo fields that predate this option compressing at 200 KB.
  if(c.type==="photo"){
    if(c.autoCompress===false)o.autoCompress=false;
    else if(clean(c.maxPhotoKB))o.maxPhotoKB=num(c.maxPhotoKB);
  }
  if(clean(c.pattern))o.pattern=c.pattern;if(clean(c.placeholder))o.placeholder=loc(c.placeholder);
  if(CHOICE.has(c.type)){const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
    if(mode==="api"&&c.optionsApi&&c.optionsApi.url){const a={url:c.optionsApi.url};["valueField","labelField","parentParam","searchParam","path","method","depKeys"].forEach(k=>{if(clean(c.optionsApi[k]))a[k]=c.optionsApi[k];});o.optionsApi=a;if(clean(c.optionsFilterBy))o.optionsFilterBy=c.optionsFilterBy;}
    else if(mode==="ref"&&clean(c.optionsRef)){o.optionsRef=c.optionsRef;if(clean(c.optionsFilterBy))o.optionsFilterBy=c.optionsFilterBy;}
    else if(c.options&&c.options.length)o.options=c.options.map(op=>{const x={value:coerce(op.value)};if(clean(op.label))x.label=loc(op.label);if(clean(op.skipTo))x.skipTo=op.skipTo;if(op.hidden)x.hidden=true;return x;});}
  ["required","readOnly","allowRemark","promptOnAdd"].forEach(k=>{if(c[k])o[k]=true;});
  if(clean(c.defaultValue))o.defaultValue=c.defaultValue;
  ["visibleWhen","enableWhen","requiredWhen"].forEach(k=>{if(clean(c[k]))o[k]=c[k];});
  if(c.skips&&c.skips.length){const s=c.skips.filter(x=>clean(x.when)||clean(x.to));if(s.length)o.skips=s.map(x=>({when:x.when,to:x.to}));}
  if(c.validations&&c.validations.length){const v=c.validations.filter(x=>clean(x.test));if(v.length)o.validations=v.map(x=>({test:x.test,message:loc(x.message||""),severity:x.severity||"error"}));}
  return o;
}
function num(v){const n=Number(v);return Number.isFinite(n)?n:v;}
function coerce(v){if(v==="true")return true;if(v==="false")return false;if(v!==""&&!isNaN(Number(v)))return Number(v);return v;}

/* ===================== VALIDATION ===================== */
function runValidation(){const issues=lint();const errs=issues.filter(i=>i.sev==="error");const h=document.getElementById("health"),t=document.getElementById("healthTxt"),tn=document.getElementById("tabN");if(errs.length){h.className="health bad";t.textContent=`${errs.length} issue${errs.length>1?"s":""}`;tn.hidden=false;tn.textContent=errs.length;}else{h.className="health ok";t.textContent="Valid";tn.hidden=true;}renderJson(issues);}
/* Per-field checks.

   These are the mistakes that used to pass with the badge still reading "Valid":
   defects that break nothing in the builder and everything in the field. The costliest
   is required + read-only, which leaves an enumerator unable to fill a question they
   cannot proceed without — that one is an error, not a warning. */
/* Leading digits are allowed: questionnaire dataKeys are routinely the question
   numbers themselves — 301, 302a, 4_umur — and nothing in the stack objects. A
   reference is lifted verbatim out of ${...} rather than tokenised as an identifier,
   and every consumer downstream treats the name as a plain string key.

   What is still rejected is what actually breaks:
     .  refResolve() splits on the first dot to address roster rows, so a dot in a
        dataKey would be read as "roster.field" and resolve to the wrong thing
     space and punctuation, which make fragile column headers and expressions
   Kept to ASCII on purpose — these names end up as export headers. */
/* Letters, digits and underscores, with dots allowed between them so questionnaire
   numbering ("3.12") can be used verbatim. Leading, trailing and doubled dots are not:
   they name nothing and would only confuse the roster.field reading. */
const DATAKEY_RE=/^\w+(\.\w+)*$/;
const DATAKEY_MAX=64;   // matches isSafeIdentifier() in internal/store/store.go
function fieldLint(c,p,add){
  const name=String(c.name||"");
  if(name&&!DATAKEY_RE.test(name))
    add("warning",p,`dataKey '${name}' should be letters, digits and underscores, with dots only between them (like 3.12). It becomes a column header and can be referenced in expressions.`);
  // The server drops any filter whose field name fails isSafeIdentifier, without
  // complaining — so an over-long dataKey quietly makes that column unfilterable on
  // the responses dashboard.
  else if(name.length>DATAKEY_MAX)
    add("warning",p,`dataKey '${name.slice(0,20)}…' is ${name.length} characters; over ${DATAKEY_MAX} the responses dashboard silently refuses to filter on it`);

  if(c.required&&c.readOnly)
    add("error",p,"the field is both required and read-only, so it can never be filled in and the response can never be completed");

  if(c.type==="calculated"&&clean(c.calculate)&&new RegExp("\\$\\{\\s*"+name.replace(/[.*+?^${}()|[\]\\]/g,"\\$&")+"\\s*[}.\\[]").test(c.calculate))
    add("error",`${p}.calculate`,`the formula refers to '${name}', its own field`);

  if(c.type!=="note"&&c.type!=="markdown"&&!clean(textOf(c.label)))
    add("warning",p,"the question has no label, so it shows up blank when filled in");

  if(CHOICE.has(c.type)){
    const mode=c.optionSource||(c.optionsApi&&c.optionsApi.url?"api":(c.optionsRef?"ref":"manual"));
    if(mode==="manual"){
      const opts=c.options||[];
      if(!opts.length)add("error",p,"a choice field with no options at all — nothing can be picked");
      else if(opts.every(o=>o.hidden))add("error",p,"every option is hidden, so the question cannot be answered");
      // A default pointing at a hidden option would preselect something the respondent
      // can neither see nor change back to.
      if(clean(c.defaultValue)&&opts.some(o=>o.hidden&&String(o.value??"")===String(c.defaultValue)))
        add("warning",p,`the default value '${c.defaultValue}' is a hidden option, so it is preselected but never shown`);
      const seen=new Map();
      opts.forEach((o,i)=>{
        const v=String(o.value??"");
        if(!clean(v)){add("warning",`${p}.option[${i}]`,"the option has no value");return;}
        if(seen.has(v))add("error",`${p}.option[${i}]`,`value '${v}' is already used by option ${seen.get(v)+1} — the answers cannot be told apart`);
        else seen.set(v,i);
      });
    }
  }
}

function lint(){
  const issues=[],add=(sev,path,msg)=>issues.push({sev,path,msg});
  const names={},fields=new Set(),containers=new Set(),exprs=[],refs=[],byName={},counts=[],rosterNames=new Set();
  const EK=["visibleWhen","enableWhen","requiredWhen","calculate"];const see=(n,p)=>{(names[n]=names[n]||[]).push(p);};
  function walk(n,base){const p=`${base}/${n.name||n.kind}`;
    if(n.kind==="field"){if(n.name){fields.add(n.name);see(n.name,p);byName[n.name]=n;}fieldLint(n,p,add);EK.forEach(k=>{if(clean(n[k]))exprs.push({path:`${p}.${k}`,expr:n[k]});});if(clean(n.optionsRef))refs.push({path:`${p}.optionsRef`,kind:"table",val:n.optionsRef});if(clean(n.optionsFilterBy))refs.push({path:`${p}.optionsFilterBy`,kind:"field",val:n.optionsFilterBy});(n.validations||[]).forEach((v,j)=>{if(clean(v.test))exprs.push({path:`${p}.val[${j}]`,expr:v.test});});(n.options||[]).forEach((o,j)=>{if(clean(o.skipTo))refs.push({path:`${p}.opsi[${j}]`,kind:"nav",val:o.skipTo});});(n.skips||[]).forEach((s,j)=>{if(clean(s.to))refs.push({path:`${p}.skip[${j}]`,kind:"nav",val:s.to});if(clean(s.when))exprs.push({path:`${p}.skip[${j}]`,expr:s.when});});}
    else{if(n.name){containers.add(n.name);see(n.name,p);if(n.kind==="roster")rosterNames.add(n.name);}(n.validations||[]).forEach((v,j)=>{if(clean(v.test))exprs.push({path:`${p}.val[${j}]`,expr:v.test});});if(n.kind==="roster"&&clean(n.countFrom)){refs.push({path:`${p}.countFrom`,kind:"field",val:n.countFrom});counts.push({path:`${p}.countFrom`,val:n.countFrom,max:n.max});}if(clean(n.visibleWhen))exprs.push({path:`${p}.visibleWhen`,expr:n.visibleWhen});(n.components||[]).forEach(c=>walk(c,p));}
  }
  state.pages.forEach(p=>walk(p,""));
  Object.entries(names).forEach(([n,ps])=>{if(ps.length>1)add("error",n,`Name '${n}' is used ${ps.length}×`);});
  const tables=new Set(Object.keys(state.referenceData||{}));const nav=new Set([...fields,...containers,"__end","__next","__prev"]);
  refs.forEach(r=>{if(r.kind==="table"&&!tables.has(r.val))add("error",r.path,`optionsRef '${r.val}' does not exist in referenceData`);if(r.kind==="field"&&!fields.has(r.val))add("error",r.path,`'${r.val}' is not an existing field`);if(r.kind==="nav"&&!nav.has(r.val))add("error",r.path,`skip target '${r.val}' not found`);});
  exprs.forEach(({path,expr})=>{try{Expr.parse(expr);}catch(e){add("error",path,"invalid expression: "+e.message);}for(const m of String(expr).matchAll(/\$\{([^}]*)\}/g)){const full=m[1].trim();const b=full.split(/[.\[]/)[0];if(!b||b.startsWith("__"))continue;if(fields.has(full)||containers.has(full))continue;if(!fields.has(b)&&!containers.has(b))add("error",path,`expression references '${b}', which does not exist`);}});
  /* Now that a dataKey may contain a dot, "3.12" can mean two things at once: the field
     of that name, or field 12 of roster 3. refResolve gives the declared field
     precedence, which is the safe way round — but the roster reading is then
     unreachable, and that is worth saying out loud rather than leaving to be
     discovered. */
  fields.forEach(fname=>{
    const dot=fname.indexOf(".");
    if(dot<0)return;
    const head=fname.slice(0,dot);
    if(rosterNames.has(head))
      add("warning",fname,`'${fname}' is both a field name and reads as roster '${head}' field '${fname.slice(dot+1)}' — expressions will use the field, and the roster form of the reference cannot be written`);
  });

  // A roster sized from another field needs that field to hold a number. Checked here
  // rather than in fieldLint because it depends on a node found elsewhere in the tree.
  counts.forEach(({path,val,max})=>{
    const t=byName[val];
    if(t&&!NUMERIC.has(t.type))
      add("warning",path,`row count comes from '${val}', which is a ${t.type} field rather than a number`);
    // Without a Max, the row count is whatever number is in that field. Pointing it at
    // a code field by mistake is easy — a village code reads 6472010 — and the form
    // then tries to build that many rows. Capped at runtime, but the instrument should
    // say what it actually expects.
    else if(t&&!clean(max))
      add("warning",path,`row count comes from '${val}' with no Max set — set one, or a mistyped answer silently asks for that many rows`);
  });

  // A reference table with no items yields an empty dropdown and says nothing about
  // why. The common cause is a leftover "source":"api" table, a form this no longer
  // supports — so it is named here rather than left to be discovered in the field.
  Object.entries(state.referenceData||{}).forEach(([k,v])=>{
    if(v&&Array.isArray(v.items)&&v.items.length)return;
    add("warning",`referenceData.${k}`,v&&v.source==="api"
      ? `table '${k}' is an API lookup, which is no longer supported — give it inline items, or move the field to the API choice source`
      : `table '${k}' has no items, so any field using it shows an empty list`);
  });
  const locs=new Set(state.locales||[]);if(state.defaultLocale&&locs.size&&!locs.has(state.defaultLocale))add("warning","defaultLocale",`default locale '${state.defaultLocale}' is missing from locales`);
  return issues;
}
function renderJson(issues){const pane=document.getElementById("paneJson");const json=serialize();const errs=issues.filter(i=>i.sev==="error"),warns=issues.filter(i=>i.sev==="warning");let ih=!issues.length?`<div class="allgood">✓ Clean — no issues found.</div>`:issues.map(it=>`<div class="issue ${it.sev==="error"?"err":"warn"}"><div><div>${esc(it.msg)}</div><div class="ipath">${esc(it.path)}</div></div></div>`).join("");pane.innerHTML=`<div class="jbar"><button class="btn" id="copyJson">Copy</button><button class="btn primary" id="dlJson">Download .json</button></div><pre class="json">${highlight(JSON.stringify(json,null,2))}</pre><div style="margin-top:14px"><div class="gh" style="font-size:11px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;color:var(--muted);margin-bottom:8px">Validation — ${errs.length} error, ${warns.length} warning</div>${ih}</div>`;pane.querySelector("#copyJson").addEventListener("click",e=>{navigator.clipboard.writeText(JSON.stringify(json,null,2));e.target.textContent="Copied ✓";setTimeout(()=>e.target.textContent="Copy",1200);});pane.querySelector("#dlJson").addEventListener("click",()=>download(`${state.id||"form"}.json`,JSON.stringify(json,null,2)));}

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
  if(obj.settings){st.settings=Object.assign(st.settings,obj.settings);st.settings.navigation=Object.assign(blankState().settings.navigation,obj.settings.navigation||{});st.settings.offline=Object.assign(blankState().settings.offline,obj.settings.offline||{});}
  st.referenceData=obj.referenceData||{};
  const pages = obj.pages || (obj.sections? obj.sections.map(s=>({kind:"page",name:s.name,title:s.title,visibleWhen:s.visibleWhen,components:s.components})) : []);
  pages.forEach(p=>st.pages.push(impNode(p,"page")));
  if(!st.pages.length)st.pages.push(blankState().pages[0]);
  state=st;selected=null;selectedSet=new Set();view={type:"page",uid:state.pages[0].uid};render();
  // Loading a different instrument starts a new history: undoing back past the load
  // into the previous instrument would be nonsense.
  History.reset();
}catch(e){alert("Import failed: "+e.message);}}
function impNode(n,forceKind){
  const kind=forceKind||n.kind||"field";
  if(kind==="page"||kind==="block"||kind==="section"){return {uid:uid(),kind,name:n.name||autoName(kind),title:textOf(n.title),visibleWhen:n.visibleWhen||"",components:(n.components||[]).map(c=>impNode(c))};}
  if(kind==="roster"){return {uid:uid(),kind:"roster",name:n.name||autoName("roster"),title:textOf(n.title),rowTitle:n.rowTitle||"",rosterType:n.rosterType||"inline",min:n.min??"",max:n.max??"",countFrom:n.countFrom||"",requiredRows:!!n.requiredRows,itemLabel:textOf(n.itemLabel),rowDefaults:textOf(n.rowDefaults),rowDisplay:n.rowDisplay||[],visibleWhen:n.visibleWhen||"",validations:(n.validations||[]).map(v=>({test:v.test||"",message:textOf(v.message),severity:v.severity||"error"})),components:(n.components||[]).map(c=>impNode(c))};}
  const f=newField(n.type||"text");f.uid=uid();f.name=n.name||f.name;f.label=textOf(n.label);f.hint=textOf(n.hint);f.html=textOf(n.html);f.markdown=textOf(n.markdown);f.calculate=n.calculate||"";f.autofill=!!n.autofill;
  ["required","readOnly","allowRemark","promptOnAdd","visibleWhen","enableWhen","requiredWhen","unit","pattern","optionsRef","optionsFilterBy","min","max","step","maxLength","maxPhotoKB","autoCompress","defaultValue"].forEach(k=>{if(n[k]!=null)f[k]=n[k];});
  f.placeholder=textOf(n.placeholder);
  if(n.options)f.options=n.options.map(o=>{const x={value:String(o.value),label:textOf(o.label),skipTo:o.skipTo||""};if(o.hidden)x.hidden=true;return x;});
  if(n.optionsApi){f.optionsApi={...n.optionsApi};f.optionSource="api";}else if(n.optionsRef){f.optionSource="ref";}else if(CHOICE.has(f.type))f.optionSource="manual";
  if(n.skips)f.skips=n.skips.map(s=>({when:s.when||"",to:s.to||""}));
  if(n.validations)f.validations=n.validations.map(v=>({test:v.test||"",message:textOf(v.message),severity:v.severity||"error"}));
  return f;
}
function textOf(v){if(v==null)return "";if(typeof v==="string")return v;if(typeof v==="object")return v[state?.defaultLocale]||v.id||Object.values(v)[0]||"";return String(v);}

/* ===================== BUILDER TOOLS (find / expression / flow) =====================
   Three review tools that all need more room than the 330px inspector, so each opens
   as an overlay. They share one modal shell and one stylesheet, injected here rather
   than added to builder.css so the whole feature stays in one place. */
(function(){
  const style=document.createElement("style");
  style.textContent=`
.bt-bg{position:fixed;inset:0;background:rgba(15,23,42,.45);z-index:120;display:flex;
  align-items:flex-start;justify-content:center;padding:60px 20px;backdrop-filter:blur(2px)}
.bt-box{background:#fff;border-radius:14px;width:100%;max-width:780px;max-height:calc(100vh - 120px);
  display:flex;flex-direction:column;box-shadow:0 24px 60px rgba(15,23,42,.28);overflow:hidden}
.bt-box.wide{max-width:960px}
.bt-head{display:flex;align-items:center;gap:10px;padding:14px 18px;border-bottom:1px solid #e6eaf0}
.bt-head h3{margin:0;font-size:15px;font-weight:700}
.bt-head .bt-x{margin-left:auto;border:none;background:transparent;font-size:19px;cursor:pointer;
  color:#64748b;line-height:1;padding:4px 8px;border-radius:6px}
.bt-head .bt-x:hover{background:#f1f5f9;color:#0f172a}
.bt-body{padding:16px 18px;overflow:auto}
.bt-in{width:100%;padding:9px 11px;border:1.5px solid #dfe4ea;border-radius:8px;font-size:13.5px;
  font-family:inherit;box-sizing:border-box}
.bt-in:focus{outline:none;border-color:#0e7490;box-shadow:0 0 0 3px rgba(14,116,144,.13)}
.bt-in.mono{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12.5px}
.bt-hint{font-size:11.5px;color:#64748b;margin:6px 0 0}
.bt-empty{color:#64748b;font-style:italic;font-size:13px;padding:14px 2px}

/* find */
.bt-hit{display:flex;align-items:center;gap:10px;padding:9px 10px;border-radius:8px;cursor:pointer;border:1px solid transparent}
.bt-hit:hover,.bt-hit.on{background:#f0f9ff;border-color:#bae6fd}
.bt-hit .k{font-family:ui-monospace,Menlo,monospace;font-size:12.5px;font-weight:700;color:#0f172a}
.bt-hit .l{font-size:12.5px;color:#475569;flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.bt-hit .t{font-size:10.5px;text-transform:uppercase;letter-spacing:.05em;color:#0e7490;
  background:#ecfeff;border:1px solid #a5f3fc;border-radius:999px;padding:2px 8px;white-space:nowrap}
.bt-hit .p{font-size:11px;color:#94a3b8;white-space:nowrap}

/* expression tester */
.bt-row{display:grid;grid-template-columns:minmax(120px,1fr) 1fr;gap:8px;margin-bottom:7px;align-items:center}
.bt-row .nm{font-family:ui-monospace,Menlo,monospace;font-size:12px;color:#334155;overflow:hidden;text-overflow:ellipsis}
.bt-res{margin-top:14px;padding:12px 14px;border-radius:9px;font-size:13px;border:1px solid #bbf7d0;background:#f0fdf4}
.bt-res.err{border-color:#fecaca;background:#fef2f2}
.bt-res .lab{font-size:10.5px;text-transform:uppercase;letter-spacing:.06em;color:#64748b;margin-bottom:4px}
.bt-res .val{font-family:ui-monospace,Menlo,monospace;font-size:13.5px;font-weight:700;word-break:break-word}

/* flow */
.bt-pg{border:1px solid #e6eaf0;border-radius:10px;padding:11px 13px;margin-bottom:9px}
.bt-pg.unreach{border-color:#fcd34d;background:#fffbeb}
.bt-pg .ttl{font-weight:700;font-size:13.5px;display:flex;align-items:center;gap:8px}
.bt-pg .num{font-family:ui-monospace,Menlo,monospace;font-size:11px;color:#94a3b8}
.bt-jump{font-size:12px;color:#475569;margin-top:6px;padding-left:2px}
.bt-jump code{font-family:ui-monospace,Menlo,monospace;background:#f1f5f9;padding:1px 5px;border-radius:4px}
.bt-jump .to{font-weight:700;color:#0f172a}
.bt-jump.back .to{color:#b45309}
.bt-jump.bad .to{color:#b91c1c}
.bt-warn{border:1px solid #fde68a;background:#fffbeb;border-radius:9px;padding:11px 14px;margin-bottom:14px;font-size:12.5px;color:#78350f}
.bt-warn ul{margin:6px 0 0;padding-left:18px}
.bt-warn li{margin:3px 0}

/* simulation */
.sim-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 18px}
.sim-out{margin-top:12px;border:1px solid #e6eaf0;border-radius:10px;overflow:hidden}
.sim-path{padding:4px 0}
.sim-step{display:flex;align-items:center;gap:10px;padding:7px 14px;font-size:12.5px;position:relative}
.sim-step+.sim-step::before{content:"";position:absolute;left:24px;top:-7px;height:7px;width:1px;background:#cbd5e1}
.sim-n{flex:none;width:17px;height:17px;border-radius:50%;background:#0e7490;color:#fff;
  font-size:10.5px;font-weight:700;display:flex;align-items:center;justify-content:center}
.sim-t{font-weight:600;color:#0f172a}
.sim-h{font-size:11px;color:#b45309;background:#fffbeb;border:1px solid #fde68a;border-radius:999px;padding:1px 8px}
.sim-step.done .sim-n{display:none}
.sim-step.done{padding-left:14px;color:#64748b}
.sim-step.done .sim-t{font-weight:600;color:#64748b}
.sim-skipped{border-top:1px solid #e6eaf0;background:#f8fafc;padding:9px 14px;font-size:11.5px;color:#64748b}
@media(max-width:760px){.sim-grid{grid-template-columns:1fr}}
`;
  document.head.appendChild(style);

  let openBox=null;
  function close(){if(openBox){openBox.remove();openBox=null;}}
  function modal(title,wide){
    close();
    const bg=document.createElement("div");bg.className="bt-bg";
    bg.innerHTML=`<div class="bt-box${wide?" wide":""}">
      <div class="bt-head"><h3>${esc(title)}</h3><button class="bt-x" aria-label="Close">✕</button></div>
      <div class="bt-body"></div></div>`;
    bg.addEventListener("click",e=>{if(e.target===bg)close();});
    bg.querySelector(".bt-x").addEventListener("click",close);
    document.body.appendChild(bg);openBox=bg;
    return bg.querySelector(".bt-body");
  }
  document.addEventListener("keydown",e=>{if(e.key==="Escape"&&openBox){close();e.stopPropagation();}},true);

  /* ---------- where does this node live ---------- */
  function pageIndexOf(uid){
    const pg=pageOf(uid);
    return pg?state.pages.findIndex(p=>p.uid===pg.uid):-1;
  }
  function trail(uid){
    const pg=pageOf(uid);
    return pg?(pg.title||pg.name):"";
  }

  /* ================= 1. FIND A FIELD ================= */
  function openFind(){
    const body=modal("Find a field");
    body.innerHTML=`<input class="bt-in" id="btFindIn" placeholder="Search by dataKey, label, or type…" autocomplete="off">
      <p class="bt-hint">Enter opens the first result. Matching is case-insensitive.</p>
      <div id="btFindOut" style="margin-top:10px"></div>`;
    const input=body.querySelector("#btFindIn"),out=body.querySelector("#btFindOut");

    const CAP=80;                                  // a longer list stops being a shortcut
    function search(q){
      q=q.trim().toLowerCase();
      if(!q)return {hits:[],total:0};
      const hits=[];let total=0;
      for(const n of allNodes()){
        if(n.kind==="page")continue;               // pages have their own list already
        const name=String(n.name||""),label=textOf(n.label||n.title||"");
        const type=n.kind==="field"?n.type:n.kind;
        const hay=`${name} ${label} ${type}`.toLowerCase();
        if(!hay.includes(q))continue;
        total++;                                   // counted past the cap, so the tally is honest
        if(hits.length<CAP)hits.push({n,name,label,type,page:trail(n.uid)});
      }
      // an exact dataKey match is almost always the one wanted
      hits.sort((a,b)=>(b.name.toLowerCase()===q)-(a.name.toLowerCase()===q));
      return {hits,total};
    }
    function paint(){
      const {hits,total}=search(input.value);
      if(!input.value.trim()){out.innerHTML=`<div class="bt-empty">Type to search across every page.</div>`;return;}
      if(!hits.length){out.innerHTML=`<div class="bt-empty">Nothing matches “${esc(input.value.trim())}”.</div>`;return;}
      out.innerHTML=hits.map((h,i)=>`<div class="bt-hit${i===0?" on":""}" data-uid="${h.n.uid}">
        <span class="k">${esc(h.name)}</span>
        <span class="l">${esc(h.label||"")}</span>
        <span class="t">${esc(h.type)}</span>
        <span class="p">${esc(h.page)}</span></div>`).join("")
        // Silently cutting the list off would let someone conclude a field is missing.
        +(total>hits.length?`<div class="bt-hint" style="padding:8px 10px 2px">Showing ${hits.length} of ${total} matches — narrow the search to see the rest.</div>`:"");
      out.querySelectorAll(".bt-hit").forEach(el=>el.addEventListener("click",()=>go(el.dataset.uid)));
    }
    function go(uid){
      const pg=pageOf(uid);
      if(pg)view={type:"page",uid:pg.uid};
      selected=uid;selectedSet=new Set([uid]);
      close();render();
      const card=document.querySelector(`#stage [data-uid="${uid}"]`);
      if(card)card.scrollIntoView({block:"center",behavior:"smooth"});
    }
    input.addEventListener("input",paint);
    input.addEventListener("keydown",e=>{
      if(e.key==="Enter"){const first=out.querySelector(".bt-hit");if(first)go(first.dataset.uid);}
    });
    paint();input.focus();
  }

  /* ================= 2. EXPRESSION TESTER ================= */
  function openExpr(){
    const body=modal("Test an expression");
    const sel=selected&&findNode(selected);
    const seed=sel?(sel.visibleWhen||sel.calculate||sel.enableWhen||sel.requiredWhen||""):"";
    body.innerHTML=`<textarea class="bt-in mono" id="btExprIn" rows="3"
        placeholder="\${age} >= 17 &amp;&amp; \${status} == 'married'">${esc(seed)}</textarea>
      <p class="bt-hint">The same evaluator the form and the preview use. Combine with <code>&amp;&amp;</code>, <code>||</code> and <code>!</code> — the words <code>and</code>, <code>or</code> and <code>not</code> are not accepted. Fill in test values below; blank means the field was left empty.</p>
      <div id="btExprVals" style="margin-top:12px"></div>
      <div id="btExprRes"></div>`;
    const input=body.querySelector("#btExprIn"),vals=body.querySelector("#btExprVals"),res=body.querySelector("#btExprRes");
    const store=Object.create(null);

    const known=new Set(allNodes().filter(n=>n.kind==="field").map(n=>n.name));
    function refsOf(src){
      const out=[];
      for(const m of String(src||"").matchAll(/\$\{([^}]*)\}/g)){
        const r=m[1].trim();
        if(r&&!out.includes(r))out.push(r);
      }
      return out;
    }
    // Delegates to the form's own coerceVal rather than repeating the rule. An earlier
    // hand-rolled version disagreed with it on "1e5", ".5", "0x10" and "Infinity",
    // which is the one thing a tester must never do: report a comparison as false that
    // the real form treats as true.
    // A blank box means unanswered, which reaches the evaluator as an absent key
    // (undefined) rather than the empty string a filled-then-cleared field would give.
    const coerce=v=>(v==null||v==="")?undefined:coerceVal(v);
    function paint(){
      const refs=refsOf(input.value);
      vals.innerHTML=refs.length
        ? refs.map(r=>`<div class="bt-row">
            <span class="nm" title="${esc(r)}">\${${esc(r)}}${known.has(r)?"":' <span style="color:#b91c1c">✕</span>'}</span>
            <input class="bt-in mono" data-ref="${esc(r)}" value="${esc(store[r]??"")}" placeholder="(empty)">
          </div>`).join("")
        : `<div class="bt-empty">No \${field} references yet.</div>`;
      vals.querySelectorAll("input[data-ref]").forEach(el=>{
        el.addEventListener("input",()=>{store[el.dataset.ref]=el.value;run();});
      });
      run();
    }
    function run(){
      const src=input.value.trim();
      if(!src){res.innerHTML="";return;}
      try{Expr.parse(src);}
      catch(e){
        res.innerHTML=`<div class="bt-res err"><div class="lab">Cannot be parsed</div><div class="val">${esc(e.message)}</div></div>`;
        return;
      }
      const missing=refsOf(src).filter(r=>!known.has(r));
      const v=Expr.evalSrc(src,name=>coerce(store[String(name).trim()]));
      const shown=v===undefined?"undefined":(typeof v==="string"?JSON.stringify(v):String(v));
      const asCond=v===undefined?"treated as true when used as a condition":`as a condition: ${!!v}`;
      res.innerHTML=`<div class="bt-res${v===undefined?" err":""}">
        <div class="lab">Result</div><div class="val">${esc(shown)}</div>
        <div class="bt-hint">${esc(asCond)}</div>
        ${missing.length?`<div class="bt-hint" style="color:#b91c1c">Not a field in this instrument: ${esc(missing.join(", "))}</div>`:""}
      </div>`;
    }
    input.addEventListener("input",paint);
    paint();input.focus();
  }

  /* ================= 3. FLOW ================= */
  /* The routing is not re-implemented here. visiblePages(), computePageSkipState() and
     pageIndexOfTarget() are the very functions the preview navigates with, so a walk
     built on them cannot disagree with the form — and a simulator that quietly
     disagreed would be worse than no simulator at all.

     They read answers from pv.values, so a run swaps that out and puts it back. */
  function withValues(vals,fn){
    const savedValues=pv.values,savedPage=pv.page;
    pv.values=Object.assign(Object.create(null),vals);
    try{return fn();}finally{pv.values=savedValues;pv.page=savedPage;}
  }

  const WALK_LIMIT=80;   // a loop in the skips must not hang the builder
  function walkPath(vals){
    return withValues(vals,()=>{
      const pages=visiblePages();
      const path=[],takenSkips=new Set();
      let i=0,guard=0,looped=false;
      while(i>=0&&i<pages.length){
        if(guard++>=WALK_LIMIT){looped=true;break;}
        pv.page=i;
        const page=pages[i];
        const st=computePageSkipState(page);
        path.push({name:page.name,title:page.title||page.name,hidden:[...st.hidden]});
        const t=st.crossPageTarget;
        if(!t){i++;continue;}
        takenSkips.add(page.name+" → "+t);
        if(t==="__end")break;
        const j=pageIndexOfTarget(t,pages);
        if(j==null){i++;continue;}
        i=j;
      }
      return {path,takenSkips,looped,reachedEnd:!looped};
    });
  }

  /* Fields any condition depends on, and the values worth trying for each.
     Candidates come from the comparisons themselves — the literal, and for numbers the
     values either side of it, which is where a boundary written the wrong way shows up
     — plus the field's own options and "unanswered". */
  function conditionSources(){
    const srcs=[];
    state.pages.forEach(p=>{if(clean(p.visibleWhen))srcs.push(p.visibleWhen);});
    for(const n of allNodes()){
      ["visibleWhen","enableWhen","requiredWhen"].forEach(k=>{if(clean(n[k]))srcs.push(n[k]);});
      (n.skips||[]).forEach(s=>{if(clean(s.when))srcs.push(s.when);});
    }
    const names=new Set();
    srcs.forEach(s=>{for(const m of String(s).matchAll(/\$\{([^}]*)\}/g)){
      const full=m[1].trim();
      if(!full||full.startsWith("__"))continue;
      /* A dataKey may contain a dot itself ("3.12"), so the whole reference is kept
         when it names a real field. Splitting first would set an answer for "3" and
         leave ${3.12} unsatisfiable — which showed up as the page being reported
         unreachable. Only when no such field exists is the name read as roster.field
         and reduced to the part in front. */
      const name=allNodes().some(x=>x.kind==="field"&&x.name===full)?full:full.split(/[.\[]/)[0];
      if(name)names.add(name);
    }});
    return {names:[...names],srcs};
  }
  function candidatesFor(name,srcs){
    const out=new Set([""]);
    const esc2=name.replace(/[.*+?^${}()|[\]\\]/g,"\\$&");
    const re=new RegExp("\\$\\{\\s*"+esc2+"\\s*\\}\\s*(==|!=|>=|<=|>|<)\\s*('[^']*'|\"[^\"]*\"|-?\\d+(?:\\.\\d+)?)","g");
    srcs.forEach(s=>{for(const m of String(s).matchAll(re)){
      const lit=m[2];
      if(/^['"]/.test(lit))out.add(lit.slice(1,-1));
      else{const v=Number(lit);out.add(String(v));out.add(String(v-1));out.add(String(v+1));}
    }});
    const node=allNodes().find(x=>x.kind==="field"&&x.name===name);
    if(node&&Array.isArray(node.options))node.options.forEach(o=>out.add(String(o.value)));
    return [...out].slice(0,6);
  }

  const COMBO_CAP=3000;
  // Deterministic, so the same instrument always gets the same verdict. A page that is
  // flagged one time and not the next would be worse than no flag at all.
  function rng(seed){return function(){seed|=0;seed=seed+0x6D2B79F5|0;
    let t=Math.imul(seed^seed>>>15,1|seed);t=t+Math.imul(t^t>>>7,61|t)^t;
    return ((t^t>>>14)>>>0)/4294967296;};}

  function explore(){
    const {names,srcs}=conditionSources();
    const cands=names.map(n=>candidatesFor(n,srcs));
    let total=1;cands.forEach(c=>{total*=Math.max(1,c.length);});
    const visitedPages=new Set(),takenSkips=new Set();
    let ran=0;
    const run=vals=>{
      ran++;
      const r=walkPath(vals);
      r.path.forEach(p=>visitedPages.add(p.name));
      r.takenSkips.forEach(s=>takenSkips.add(s));
    };

    /* Plain recursion down the candidate lists looks exhaustive but is not, once the
       cap bites: it exhausts the last field while the first stays pinned to its first
       candidate. On a 14-field instrument that left `${a0} == 7` never tried, and six
       plainly reachable pages were reported unreachable. So the sweep comes first and
       the enumeration second. */
    run({});                                              // nobody answered anything
    names.forEach((n,i)=>cands[i].forEach(v=>{if(v!=="")run({[n]:v});}));  // one field at a time

    if(total<=COMBO_CAP){
      (function rec(i,vals){
        if(i>=names.length){run(vals);return;}
        for(const v of cands[i])rec(i+1,Object.assign({},vals,{[names[i]]:v}));
      })(0,{});
    }else{
      // Too many to enumerate: sample the space instead of marching through one corner
      // of it, so combinations of conditions still get a fair chance.
      const rand=rng(0x5E2026);
      while(ran<COMBO_CAP){
        const vals={};
        names.forEach((n,i)=>{const c=cands[i],v=c[Math.floor(rand()*c.length)];if(v!=="")vals[n]=v;});
        run(vals);
      }
    }
    return {visitedPages,takenSkips,ran,capped:total>COMBO_CAP,total,names,cands};
  }

  function openFlow(){
    const body=modal("Flow — simulate a path, and check what can be reached",true);
    const pages=state.pages;
    const idxByName={},nameOfPage={};
    pages.forEach((p,i)=>{idxByName[p.name]=i;nameOfPage[p.uid]=p.name;});

    // Every skip in the instrument, resolved to the page it lands on.
    const jumps=[];
    for(const n of allNodes()){
      if(!n.skips||!n.skips.length)continue;
      const from=pageIndexOf(n.uid);
      if(from<0)continue;
      for(const s of n.skips){
        const to=String(s.to||"").trim();
        if(!to)continue;
        let target=-1,kind="page";
        if(to==="__end"){kind="end";}
        else if(to==="__next"||to==="__prev"){kind="rel";}
        else if(idxByName[to]!==undefined){target=idxByName[to];}
        else{
          // a skip may also name a field or container: resolve it to its page
          const node=allNodes().find(x=>x.name===to);
          if(node){target=pageIndexOf(node.uid);kind="into";}
          else kind="missing";
        }
        jumps.push({from,to,target,kind,field:n.name,when:s.when||""});
      }
    }

    // Which pages a jump can vault over. Pages only ever reached by falling through
    // are fine; the interesting ones are those a jump can skip past entirely.
    const bypassable=new Set();
    jumps.forEach(j=>{if(j.target>j.from+1)for(let i=j.from+1;i<j.target;i++)bypassable.add(i);});
    const backward=jumps.filter(j=>j.target>=0&&j.target<=j.from);
    const missing=jumps.filter(j=>j.kind==="missing");

    const warn=[];
    if(missing.length)warn.push(missing.length===1
      ? `1 skip names a target that does not exist: ${esc(missing[0].to)}`
      : `${missing.length} skips name targets that do not exist: ${esc([...new Set(missing.map(m=>m.to))].join(", "))}`);
    if(backward.length)warn.push(backward.length===1
      ? `1 skip jumps backwards or onto its own page — check it cannot loop.`
      : `${backward.length} skips jump backwards or onto their own page — check they cannot loop.`);
    if(!jumps.length)warn.push("No skips are defined, so the instrument runs straight through in page order.");

    const arrow=j=>{
      const cls=j.kind==="missing"?"bad":(j.target>=0&&j.target<=j.from?"back":"");
      const dest=j.kind==="end"?"end of form":(j.kind==="rel"?j.to:(j.target>=0?`${j.target+1}. ${pages[j.target].title||pages[j.target].name}`:j.to));
      return `<div class="bt-jump ${cls}"><code>${esc(j.field)}</code> → <span class="to">${esc(dest)}</span>${j.when?` <span style="color:#94a3b8">when ${esc(j.when)}</span>`:""}</div>`;
    };

    /* ---- run the exploration up front: it answers the question the static map could
       not, which is whether a page or a branch can ever actually happen ---- */
    const ex=explore();
    const neverVisited=pages.filter(p=>!ex.visitedPages.has(p.name));
    const neverTaken=jumps.filter(j=>j.kind!=="missing"&&![...ex.takenSkips].some(k=>k.endsWith(" → "+j.to)));

    // Careful not to claim the pages therefore run in order: an unconditional skip
    // reorders them without depending on any answer at all.
    if(ex.names.length===0)warn.push("No condition depends on an answer, so there is exactly one path through this instrument — shown below.");
    if(neverVisited.length)warn.push(`${neverVisited.length} page${neverVisited.length>1?"s were":" was"} never reached in ${ex.ran} simulated run${ex.ran>1?"s":""}: ${esc(neverVisited.map(p=>p.title||p.name).join(", "))}`);
    if(neverTaken.length)warn.push(`${neverTaken.length} skip${neverTaken.length>1?"s":""} never fired in any run — the condition may be impossible: ${esc(neverTaken.map(j=>j.field+" → "+j.to).join(", "))}`);

    const simFields=ex.names;
    const inputFor=n=>{
      const node=allNodes().find(x=>x.kind==="field"&&x.name===n);
      if(node&&CHOICE.has(node.type)&&Array.isArray(node.options)&&node.options.length)
        return `<select class="bt-in mono" data-sim="${esc(n)}"><option value="">(unanswered)</option>${
          node.options.map(o=>`<option value="${esc(o.value)}">${esc(String(o.value))} — ${esc(textOf(o.label)||"")}</option>`).join("")}</select>`;
      return `<input class="bt-in mono" data-sim="${esc(n)}" placeholder="(unanswered)">`;
    };

    body.innerHTML=`
      ${warn.length?`<div class="bt-warn"><strong>What to look at</strong><ul>${warn.map(w=>`<li>${w}</li>`).join("")}</ul></div>`:""}

      <div class="gh" style="margin:2px 0 8px">Simulate a respondent</div>
      ${simFields.length
        ? `<div class="sim-grid">${simFields.map(n=>`<div class="bt-row"><span class="nm" title="${esc(n)}">${esc(n)}</span>${inputFor(n)}</div>`).join("")}</div>`
        : `<div class="bt-empty" style="padding:6px 2px 10px">Nothing here reads an answer, so there is only one path — and it is worth seeing, because an unconditional skip can still reorder or loop the pages.</div>`}
      <div id="simOut" class="sim-out"></div>

      <div class="gh" style="margin:20px 0 8px">Every skip that is declared</div>
      ${pages.map((p,i)=>{
        const out=jumps.filter(j=>j.from===i);
        const unreached=!ex.visitedPages.has(p.name);
        return `<div class="bt-pg${unreached?" unreach":""}">
          <div class="ttl"><span class="num">${i+1}</span> ${esc(p.title||p.name)}
            ${unreached?`<span class="t" style="font-size:10px;background:#fef3c7;border-color:#fcd34d;color:#92400e">never reached</span>`:""}
            ${bypassable.has(i)&&!unreached?`<span class="t" style="font-size:10px">can be skipped past</span>`:""}</div>
          ${out.length?out.map(arrow).join(""):`<div class="bt-jump" style="color:#94a3b8">falls through to the next page</div>`}
        </div>`;
      }).join("")}

      <p class="bt-hint">${ex.ran} run${ex.ran===1?"":"s"} simulated${ex.capped?` (capped — ${ex.total.toLocaleString()} combinations exist)`:""}, varying only the ${ex.names.length} field${ex.names.length===1?"":"s"} the conditions depend on, using values taken from those conditions. That is evidence, not proof: a page reported as never reached may still be reachable through a value this did not try.</p>`;

    /* ---- live simulation ---- */
    const out=body.querySelector("#simOut");
    function runSim(){
      const vals={};
      body.querySelectorAll("[data-sim]").forEach(el=>{if(el.value!=="")vals[el.dataset.sim]=el.value;});
      const r=walkPath(vals);
      const visited=new Set(r.path.map(p=>p.name));
      out.innerHTML=`
        <div class="sim-path">${r.path.map((p,i)=>`
          <div class="sim-step">
            <span class="sim-n">${i+1}</span>
            <span class="sim-t">${esc(p.title)}</span>
            ${p.hidden.length?`<span class="sim-h">${p.hidden.length} field${p.hidden.length>1?"s":""} hidden by a skip</span>`:""}
          </div>`).join("")}
          <div class="sim-step done">${r.looped
            ? `<span class="sim-t" style="color:#b91c1c">stopped after ${WALK_LIMIT} pages — the skips appear to loop</span>`
            : `<span class="sim-t">end of form</span>`}</div>
        </div>
        ${pages.filter(p=>!visited.has(p.name)).length
          ? `<div class="sim-skipped">Not shown on this path: ${esc(pages.filter(p=>!visited.has(p.name)).map(p=>p.title||p.name).join(", "))}</div>`
          : `<div class="sim-skipped">Every page is shown on this path.</div>`}`;
    }
    body.querySelectorAll("[data-sim]").forEach(el=>{
      el.addEventListener("input",runSim);el.addEventListener("change",runSim);
    });
    // Always, even with nothing to vary: that is exactly the case where the single
    // path is worth showing, and where an unconditional skip loop would otherwise
    // leave the panel blank.
    runSim();
  }

  document.getElementById("btnFind").addEventListener("click",openFind);
  document.getElementById("btnExpr").addEventListener("click",openExpr);
  document.getElementById("btnFlow").addEventListener("click",openFlow);
  document.getElementById("btnUndo").addEventListener("click",()=>History.undo());
  document.getElementById("btnRedo").addEventListener("click",()=>History.redo());
  document.addEventListener("keydown",e=>{
    if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="f"&&document.getElementById("preview").hidden){
      e.preventDefault();openFind();
    }
  });
})();

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
document.getElementById("addPage").addEventListener("click",()=>{const p=newPage();p.title="Page "+(state.pages.length+1);state.pages.push(p);view={type:"page",uid:p.uid};selected=p.uid;render();});
document.getElementById("btnExport").addEventListener("click",()=>{switchTab("json");download(`${state.id||"form"}.json`,JSON.stringify(serialize(),null,2));});
document.getElementById("btnImport").addEventListener("click",()=>document.getElementById("fileInput").click());
document.getElementById("fileInput").addEventListener("change",e=>{const f=e.target.files[0];if(!f)return;const r=new FileReader();r.onload=()=>{try{importJSON(JSON.parse(r.result));}catch(err){alert("Invalid JSON: "+err.message);}};r.readAsText(f);e.target.value="";});
document.getElementById("btnPreview").addEventListener("click",openPreview);
document.getElementById("pvClose").addEventListener("click",closePreview);
document.getElementById("pvMode").addEventListener("change",e=>{pv.mode=e.target.value;pv.page=0;renderPreview();});
document.addEventListener("keydown",e=>{
  if(e.key==="Escape"&&!document.getElementById("preview").hidden){closePreview();return;}
  const at=document.activeElement,tag=at&&at.tagName;
  if(tag==="INPUT"||tag==="TEXTAREA"||(at&&at.isContentEditable))return; // do not interfere with ordinary text copy-paste
  if(e.key==="Escape"&&selectedSet.size>0){selected=null;selectedSet=new Set();render();return;}
  if((e.key==="Delete"||e.key==="Backspace")&&selectedSet.size>0&&document.getElementById("preview").hidden){
    e.preventDefault();
    if(selectedSet.size>1){
      if(confirm(`Delete the ${selectedSet.size} selected items?`)){[...selectedSet].forEach(uid=>removeNode(uid));selected=null;selectedSet=new Set();render();}
    }else if(selected){
      if(confirm("Delete this?")){removeNode(selected);selected=null;selectedSet=new Set();render();}
    }
    return;
  }
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="c"&&selected){const n=findNode(selected);if(n){copyNode(n);render();e.preventDefault();}}
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="v"&&clipboard){pasteNode();e.preventDefault();}
  // Deliberately below the INPUT/TEXTAREA guard above: while the caret is in a text
  // box, Ctrl+Z belongs to that box. Click out and it undoes the instrument instead;
  // the toolbar buttons work regardless of where focus is.
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="z"){e.preventDefault();e.shiftKey?History.redo():History.undo();return;}
  if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==="y"){e.preventDefault();History.redo();return;}
});
document.querySelector(".canvas").addEventListener("click",e=>{if(e.target.classList.contains("canvas")||e.target.closest(".cv-head")&&!e.target.closest("button")){selected=null;selectedSet=new Set();render();}});

/* ===================== PREVIEW (View Form) ===================== */
let pv={values:{},page:0,mode:"section",row:null};
function openPreview(){pv={values:{},page:0,mode:state.settings.navigation.mode==="scroll"?"scroll":"section",row:null,apiCache:{}};const sel=document.getElementById("pvMode");if(sel)sel.value=pv.mode;document.getElementById("preview").hidden=false;renderPreview();setupSidebarToggle(document.getElementById("pvSide"));}
function closePreview(){document.getElementById("preview").hidden=true;const s=document.getElementById("pvSide");if(s){s.classList.remove("open","collapsed");}document.getElementById("pvSideBd")?.classList.remove("show");}
function setupSidebarToggle(side){
  const tog=document.getElementById("pvSideTog");
  const bd=document.getElementById("pvSideBd");
  if(!tog||!side)return;
  tog.replaceWith(tog.cloneNode(true)); // drop the old listener
  const newTog=document.getElementById("pvSideTog");
  const mobile=()=>window.innerWidth<=680;
  newTog.addEventListener("click",()=>{
    if(mobile()){const open=side.classList.toggle("open");if(bd)bd.classList.toggle("show",open);}
    else{side.classList.toggle("collapsed");}
  });
  if(bd){bd.replaceWith(bd.cloneNode(true));document.getElementById("pvSideBd").addEventListener("click",()=>{side.classList.remove("open");document.getElementById("pvSideBd").classList.remove("show");});}
}
function coerceVal(v){if(v==="true")return true;if(v==="false")return false;if(typeof v==="string"&&v.trim()!==""&&!isNaN(Number(v)))return Number(v);return v;}
function ancestorPrefixes(rowPrefix){
  const out=[];
  let p=String(rowPrefix||"");
  while(p){
    out.push(p);
    const m=p.match(/^(.*?)(?:[^#]+#\d+#)$/);
    if(!m||m[1]===p)break;
    p=m[1];
  }
  return out;
}
function resolveScopedValue(name,rowPrefix){
  for(const p of ancestorPrefixes(rowPrefix)){
    const k=p+name;
    if(k in pv.values)return coerceVal(pv.values[k]);
  }
  return undefined;
}
function refResolve(name,rowPrefix){
  name=String(name).trim();
  if(name.includes(".")){
    /* A dataKey may legitimately contain a dot — questionnaire numbering runs "3.12".
       A field declared under that exact name therefore wins over reading the text as
       roster.field, so that naming a roster "3" later cannot silently steal every
       ${3.12} reference and turn it into an empty array.

       Decided from the schema rather than from the answers on purpose: keying off
       pv.values would make the same expression resolve differently before and after
       the respondent fills the field in. lint() reports the collision separately. */
    if(!allNodes().some(x=>x.kind==="field"&&x.name===name)){
      const dot=name.indexOf(".");const rn=name.slice(0,dot),fn=name.slice(dot+1);
      const r=allNodes().find(x=>x.kind==="roster"&&x.name===rn);
      if(r){const cnt=rosterCount(r);const arr=[];for(let i=0;i<cnt;i++)arr.push(coerceVal(pv.values[`${rn}#${i}#${fn}`]));return arr;}
    }
    if(rowPrefix){const v=resolveScopedValue(name,rowPrefix);if(v!==undefined)return v;}
    return name in pv.values?coerceVal(pv.values[name]):undefined;
  }
  const rn=allNodes().find(x=>x.kind==="roster"&&x.name===name);
  if(rn){const cnt=rosterCount(rn);return Array.from({length:cnt},(_,i)=>i);}
  if(rowPrefix){const v=resolveScopedValue(name,rowPrefix);if(v!==undefined)return v;}
  return coerceVal(pv.values[name]);
}
function evalExprSrc(src,rowPrefix){return Expr.evalSrc(src,name=>refResolve(name,rowPrefix||""));}
function evalVisible(src,rowPrefix){if(!src)return true;const v=evalExprSrc(src,rowPrefix||"");return v===undefined?true:!!v;}
function pvEmpty(v){return v==null||v===""||(Array.isArray(v)&&v.length===0);}
/* A formula over answers that have not been given yet evaluates to NaN — ${a} + ${b}
   with both blank is undefined + undefined — and a division by zero gives Infinity.
   Neither is an answer, and neither may be written into the response: an autofill field
   only recomputes while it is empty, so storing "NaN" once left it stuck there for
   good, even after the operands were filled in correctly. */
function calcUsable(r){return r!==undefined&&r!==""&&!(typeof r==="number"&&!Number.isFinite(r));}
/* Values only this code could have written, from before the guard above existed.
   Treated as "not filled in yet" so an in-progress form repairs itself. */
function calcPoisoned(v){return v==="NaN"||v==="Infinity"||v==="-Infinity"||(typeof v==="number"&&!Number.isFinite(v));}
function refLabels(ref,parentVal,filterField){const tbl=state.referenceData&&state.referenceData[ref];if(!tbl||!tbl.items)return [];return tbl.items.filter(it=>{if(filterField&&parentVal!=null&&parentVal!=="")return String(it.parent)===String(parentVal);return true;}).map(it=>({value:it.code,label:textOf(it.label)}));}
function pvOptions(c,rowPrefix){if(c.optionsRef){const pVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rowPrefix):null;return refLabels(c.optionsRef,pVal,c.optionsFilterBy);}return (c.options||[]).filter(o=>!o.hidden).map(o=>({value:o.value,label:textOf(o.label)||String(o.value)}));}
function getPath(obj,path){return String(path).split(".").reduce((o,k)=>(o==null?o:o[k]),obj);}
function buildApiUrl(tbl,parentVal,rp){
  rp=rp||"";
  let url=tbl.url;
  // Replace {key}: {parent} → parentVal; other fields → look for the roster-prefixed one first, then the page
  url=url.replace(/\{([^}]+)\}/g,(_,k)=>encodeURIComponent(k==="parent"?(parentVal??""): ((rp&&pv.values[rp+k]!=null?pv.values[rp+k]:pv.values[k])??"")));
  if(!tbl.url.includes("{")&&tbl.parentParam&&parentVal!=null&&parentVal!=="")
    url+=(url.includes("?")?"&":"?")+encodeURIComponent(tbl.parentParam)+"="+encodeURIComponent(parentVal);
  return url;
}
function apiFetch(tbl,parentVal,rp){
  pv.apiCache=pv.apiCache||{};const url=buildApiUrl(tbl,parentVal,rp);let e=pv.apiCache[url];
  if(!e){e=pv.apiCache[url]={state:"loading",opts:[]};
    fetch("/api/options-proxy?url="+encodeURIComponent(url))
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
    // Dependencies: {key} placeholders in the URL (other than {parent}), plus any
    // dataKeys named explicitly in depKeys.
    const urlDeps=(cfg.url.match(/\{([^}]+)\}/g)||[]).map(m=>m.slice(1,-1)).filter(k=>k!=="parent");
    const explicitDeps=(cfg.depKeys||"").split(",").map(s=>s.trim()).filter(Boolean);
    const deps=[...new Set([...urlDeps,...explicitDeps])];
    const missing=deps.find(k=>{const v=rp?(pv.values[rp+k]??pv.values[k]):pv.values[k];return v==null||v==="";});
    if(missing)return {state:"skip",opts:[]};
    const parentVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rp):null;
    const e=apiFetch(cfg,parentVal,rp);return {state:e.state==="done"?"ok":e.state,opts:e.opts||[],error:e.error};
  }
  // Reference tables are inline only. They used to accept a "source":"api" form as
  // well, but the form-filling page never implemented it: such a table previewed fine
  // here and then yielded no options at all in the field. Removed rather than
  // completed — a field that needs a live service already has its own API source.
  if(mode==="ref"&&c.optionsRef){const tbl=state.referenceData&&state.referenceData[c.optionsRef];if(!tbl)return {state:"ok",opts:[]};const parentVal=c.optionsFilterBy?refResolve(c.optionsFilterBy,rp):null;return {state:"ok",opts:refLabels(c.optionsRef,parentVal,c.optionsFilterBy)};}
  // Hidden options are filtered out of what can be picked, never out of the schema:
  // answers already recorded against them still resolve to a label everywhere else.
  return {state:"ok",opts:(c.options||[]).filter(o=>!o.hidden).map(o=>({value:o.value,label:textOf(o.label)||String(o.value)}))};
}
function optWrap(ro,fn,key){
  if(ro.state==="skip"){if(key!=null)pv.values[key]="";return '<select class="pv-in" disabled><option value="">— fill in the previous field first —</option></select>';}
  if(ro.state==="loading")return '<div class="pv-loading">⏳ Loading options from the API…</div>';
  if(ro.state==="error")return `<div class="pv-vmsg error">Failed to load options: ${esc(ro.error||"")}</div>`;
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
// The first skip target registered on this field (used as the default
// when the field has not been answered at all — see fieldEffectiveSkipTarget).
function fieldPendingTarget(f){
  const s=(f.skips||[]).find(x=>x.when&&x.to);
  if(s)return s.to;
  if(CHOICE.has(f.type)&&Array.isArray(f.options)){
    // Only among options that can actually be picked — see fieldPendingTarget in
    // public.html.
    const o=f.options.find(o=>o.skipTo&&!o.hidden);
    if(o)return o.skipTo;
  }
  return null;
}
// A field that can skip but has NOT been answered yet counts as "pending":
// by default the fields between it and the target stay hidden until
// answered with an option that does NOT trigger a skip (only then does the normal path open).
function fieldEffectiveSkipTarget(f){
  const t=fieldSkipTarget(f);
  if(t)return t;
  if(pvEmpty(pv.values[f.name]))return fieldPendingTarget(f);
  return null;
}
// Roster variant: evaluate the skip using one specific row's answers (rp = "roster#idx#").
function fieldSkipTargetRp(f,rp){
  for(const s of (f.skips||[])){
    if(!s.when||!s.to)continue;
    const r=evalExprSrc(s.when,rp);
    if(r===undefined)continue;
    if(r)return s.to;
  }
  if(CHOICE.has(f.type)&&Array.isArray(f.options)){
    const v=pv.values[rp+f.name];
    const sel=Array.isArray(v)?v.map(String):[String(v)];
    for(const o of f.options){if(o.skipTo&&sel.includes(String(o.value)))return o.skipTo;}
  }
  return null;
}
function fieldEffectiveSkipTargetRp(f,rp){
  const t=fieldSkipTargetRp(f,rp);
  if(t)return t;
  if(pvEmpty(pv.values[rp+f.name]))return fieldPendingTarget(f);
  return null;
}
// Work out which fields in one roster row must be hidden because of a skip-to.
// The "__end" target means finish the rest of this row (later fields in the same
// row are hidden; other roster rows are unaffected).
function computeRosterRowSkipState(roster,rowIdx){
  const rp=`${roster.name}#${rowIdx}#`;
  const allRowFields=flatFields(roster.components);
  const allNames=new Set(allRowFields.map(f=>f.name));
  const hidden=new Set();
  let skipActive=false,skipTarget=null;
  for(const f of allRowFields){
    if(skipActive){
      if(skipTarget&&f.name===skipTarget){skipActive=false;skipTarget=null;}
      else{hidden.add(f.name);continue;}
    }
    const t=fieldEffectiveSkipTargetRp(f,rp);
    if(t==="__end"){skipActive=true;skipTarget=null;}
    else if(t&&allNames.has(t)&&t!==f.name){skipActive=true;skipTarget=t;}
  }
  return hidden;
}
// Work out which fields on this page must be hidden because a skip is currently
// active (its target is on the same page), plus the cross-page target when the
// active skip has not "finished" by the end of the page.
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
let ROW_SKIP_HIDDEN=new Map();
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
/* Rules attached to the roster itself rather than to a field inside it: ${name}
   resolves to the roster's list of rows, so len(${name}) is the count and can be
   compared with another answer. Evaluated even when nothing has been entered — a
   roster always has a value, and an empty one is a count of zero.
   Mirrors rosterFailedRules() in public.html. */
function rosterFailedRules(r,rowPrefix,blockingOnly){
  const out=[];
  (r.validations||[]).forEach(v=>{
    if(!v.test)return;
    if(blockingOnly&&v.severity==="warning")return;
    const res=evalExprSrc(v.test,rowPrefix||"");
    if(res===undefined)return;   // unparseable or unknown reference — lint reports it
    if(!res)out.push(v);
  });
  return out;
}
function rosterValidationMsgs(r,rowPrefix){
  return rosterFailedRules(r,rowPrefix,false).map(v=>
    `<div class="pv-vmsg ${v.severity==="warning"?"warning":"error"}">${esc(textOf(v.message)||"The number of rows is not valid")}</div>`
  ).join("");
}
function validateCurrentPage(page){
  const targets=pageValidationTargets(page);
  for(const {c,rp} of targets){
    if(c.type==="note"||c.type==="markdown"||c.type==="hidden"||(c.type==="calculated"&&!c.autofill))continue;
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
  // Roster-level rules. The preview has to enforce them too, or the designer cannot
  // tell whether the rule they just wrote actually works.
  let gate=null;
  (function walkR(comps,pfx){if(gate)return;(comps||[]).forEach(c=>{
    if(gate||!evalVisible(c.visibleWhen,pfx))return;
    if(c.kind==="roster"){if(rosterFailedRules(c,pfx,true).length)gate={ok:false,key:"pvroster_"+c.name};}
    else if(c.kind!=="field"&&c.components)walkR(c.components,pfx);
  });})(page.components||[],"");
  if(gate)return gate;
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
  const navArea=document.getElementById("pvNavArea");
  document.getElementById("pvTitle").textContent=textOf(state.title)||"Form";
  if(pv.row){renderRosterRowPage();renderPvSide();return;}
  const pages=visiblePages();
  if(!pages.length){body.innerHTML=`<div class="pv-empty">No pages to show yet.</div>`;if(navArea)navArea.innerHTML="";renderPvSide();return;}
  if(pv.mode==="scroll"){
    SKIP_HIDDEN=new Set();ROW_SKIP_HIDDEN=new Map();
    let h="";pages.forEach(p=>h+=pvPage(p));
    body.innerHTML=h;if(navArea)navArea.innerHTML="";
  } else {
    if(pv.page>=pages.length)pv.page=pages.length-1;const p=pages[pv.page];
    SKIP_HIDDEN=computePageSkipState(p).hidden;
    SKIP_HIDDEN.forEach(name=>{delete pv.values[name];});
    ROW_SKIP_HIDDEN=new Map();
    body.innerHTML=pvPage(p);
    if(navArea){
      const pct=pages.length>1?Math.round(((pv.page+1)/pages.length)*100):100;
      navArea.innerHTML=`<div class="pv-nav-err" id="pvNavErr"></div><div class="pv-nav">${pv.page>0?`<button class="btn" id="pvPrev">← Sebelumnya</button>`:`<span></span>`}<div class="pv-prog-wrap"><div class="pv-progbar-track"><div class="pv-progbar" style="width:${pct}%"></div></div><span class="pv-prog">Page ${pv.page+1} / ${pages.length}</span></div>${pv.page<pages.length-1?`<button class="btn primary" id="pvNext">Lanjut →</button>`:`<button class="btn primary" id="pvDone">Finish</button>`}</div>`;
    }
  }
  bindPreview(body);body.scrollTop=keep;renderPvSide();
  document.getElementById("pvPrev")?.addEventListener("click",()=>{pv.page--;body.scrollTop=0;renderPreview();});
  document.getElementById("pvNext")?.addEventListener("click",()=>{
    const curP=pages[pv.page];
    const gate=validateCurrentPage(curP);
    if(!gate.ok){const err=document.getElementById("pvNavErr");if(err)err.textContent="Complete the required questions / fix the invalid entries before continuing.";focusPvField(gate.key);return;}
    const target=computePageSkipState(curP).crossPageTarget;
    const idx=target?pageIndexOfTarget(target,pages):null;
    if(idx!=null){clearSkippedPages(pages,pv.page,idx);pv.page=idx;body.scrollTop=0;renderPreview();return;}
    pv.page++;body.scrollTop=0;renderPreview();
  });
  document.getElementById("pvDone")?.addEventListener("click",()=>{
    const curP=pages[pv.page];
    const gate=validateCurrentPage(curP);
    if(!gate.ok){const err=document.getElementById("pvNavErr");if(err)err.textContent="Complete the required questions / fix the invalid entries before submitting.";focusPvField(gate.key);return;}
    alert("Preview finished. This is display only — nothing is saved.");
  });
}
function pvPage(p){let h=`<div class="pv-page" id="pvpage_${esc(p.name)}"><h2 class="pv-h2">${esc(p.title||p.name)}</h2>`;p.components.forEach(c=>h+=pvNode(c,null));return h+`</div>`;}
function pvNode(c,row){
  const rp=rowStoragePrefix(row);
  if(c.kind==="block"){if(!evalVisible(c.visibleWhen,rp))return "";const inner=(c.components||[]).map(x=>pvNode(x,row)).join("");if(!inner)return "";let h=`<div class="pv-card">`;if(c.title)h+=`<div class="pv-bt">${esc(rowInterp(c.title,row))}</div>`;return h+inner+`</div>`;}
  if(c.kind==="section"){if(!evalVisible(c.visibleWhen,rp))return "";const inner=(c.components||[]).map(x=>pvNode(x,row)).join("");if(!inner)return "";let h=`<div class="pv-sec">`;if(c.title)h+=`<div class="pv-st">${esc(rowInterp(c.title,row))}</div>`;return h+inner+`</div>`;}
  if(c.kind==="roster"){if(!evalVisible(c.visibleWhen,rp))return "";return pvRoster(c,row);}
  return pvField(c,row);
}
/* An absolute ceiling on rows, whatever the answers say — Max on the roster is
   optional, so without this a countFrom pointing at a code field by mistake asked the
   browser to build millions of rows. Mirrors ROSTER_MAX_ROWS in public.html. */
const ROSTER_MAX_ROWS=500;
function rosterCount(r,rowPrefix){return Math.min(rosterCountRaw(r,rowPrefix),ROSTER_MAX_ROWS);}
function rosterCountRaw(r,rowPrefix){
  const from=String(r.countFrom||"").trim();
  if(from){
    let raw=refResolve(from,rowPrefix||"");
    if((raw==null||raw==="")&&pv.row){
      const parent=findNode(pv.row.uid);
      if(parent){
        const k=`${parent.name}#${pv.row.index}#${from}`;
        if(k in pv.values)raw=pv.values[k];
      }
    }
    let cf=Number(raw);
    if(!Number.isFinite(cf)||cf<0)cf=0;
    cf=Math.floor(cf);
    if(r.max!==""&&r.max!=null&&cf>Number(r.max))cf=Number(r.max);
    return cf;
  }
  const k=rosterCountKey(r,rowPrefix);
  if(pv.values[k]==null)pv.values[k]=Number(r.min)||0;
  let c=Number(pv.values[k])||0;
  if(r.max!==""&&r.max!=null){
    const mx=Number(r.max);
    if(Number.isFinite(mx)&&mx>=0&&c>mx){
      c=mx;
      pv.values[k]=mx;
    }
  }
  return c;
}
function labelOfField(name){const n=allNodes().find(x=>x.kind==="field"&&x.name===name);return n?(n.label||n.name):name;}
function flatFields(comps){const out=[];function go(a){(a||[]).forEach(c=>{c.kind==="field"?out.push(c):c.components&&go(c.components);});}go(comps);return out;}
function rowSummary(r,i){
  const disp=(r.rowDisplay&&r.rowDisplay.length)?r.rowDisplay:flatFields(r.components).slice(0,1).map(c=>c.name);
  const parts=disp.map(fn=>{const v=pv.values[`${r.name}#${i}#${fn}`];return (v==null||v==="")?null:String(v);}).filter(Boolean);
  return parts.length?esc(parts.join(" · ")):"";
}
function isRowFilled(r,i){return flatFields(r.components).some(f=>{const v=pv.values[`${r.name}#${i}#${f.name}`];return v!=null&&v!=="";});}
function primaryRowField(r){if(r.rowDisplay&&r.rowDisplay.length)return r.rowDisplay[0];const f=flatFields(r.components)[0];return f?f.name:null;}
function rowDefaultValue(r,i){
  const raw=String(r.rowDefaults||"");
  if(!raw)return "";
  const lines=raw.split(/\r?\n/).map(s=>s.trim());
  return (i<lines.length&&lines[i])?lines[i]:"";
}
function ensureRosterDefaultValues(r,count){
  const pf=primaryRowField(r);
  if(!pf) return;
  for(let i=0;i<count;i++){
    const key=`${r.name}#${i}#${pf}`;
    const cur=pv.values[key];
    if(cur!=null&&String(cur)!=="") continue;
    const def=rowDefaultValue(r,i);
    if(def) pv.values[key]=def;
  }
}
function openAddRowModal(r,hostRp){
  const scope=rosterScopePrefix(r,hostRp||"");   // "" or "art#0#" + name + "#"
  const title=r.rowTitle||"row";
  const promptFields=flatFields(r.components).filter(f=>f.promptOnAdd);
  const bg=document.createElement("div");bg.className="pv-modal-bg";
  const fieldsHtml=promptFields.length
    ?promptFields.map(f=>`<div style="margin-bottom:10px"><label style="display:block;font-size:12.5px;font-weight:500;margin-bottom:5px">${esc(f.label||f.name)}</label><input class="pv-modal-in" data-pf="${esc(f.name)}" placeholder="${esc(f.placeholder||f.label||f.name)}…" style="width:100%;border:1px solid var(--line);border-radius:var(--radius-s);padding:9px 10px;font-size:13px;box-sizing:border-box"></div>`).join("")
    :`<input type="text" id="addRowInput" placeholder="${esc(title)}…" style="width:100%;border:1px solid var(--line);border-radius:var(--radius-s);padding:9px 10px;font-size:13px;margin-bottom:4px;box-sizing:border-box">`;
  bg.innerHTML=`<div class="pv-modal"><h3>Add ${esc(title)}</h3>${fieldsHtml}<div class="pv-modal-actions"><button class="btn ghost" id="addRowCancel">Cancel</button><button class="btn primary" id="addRowConfirm">+ Add ${esc(title)}</button></div></div>`;
  document.body.appendChild(bg);
  const firstInput=bg.querySelector("input");if(firstInput)firstInput.focus();
  const close=()=>bg.remove();
  bg.querySelector("#addRowCancel").addEventListener("click",close);
  bg.addEventListener("click",e=>{if(e.target===bg)close();});
  const confirmAdd=()=>{
    const key=`${scope}count`;
    const cur=Number(pv.values[key])||0;
    if(r.max!==""&&r.max!=null){
      const mx=Number(r.max);
      if(Number.isFinite(mx)&&mx>=0&&cur>=mx){
        alert(`Maximum ${mx} rows.`);
        return;
      }
    }
    if(cur>=ROSTER_MAX_ROWS){alert(`This form allows at most ${ROSTER_MAX_ROWS} rows here.`);return;}
    const idx=cur;
    pv.values[key]=idx+1;
    if(promptFields.length){
      bg.querySelectorAll("[data-pf]").forEach(el=>{const v=el.value.trim();if(v)pv.values[`${scope}${idx}#${el.dataset.pf}`]=v;});
    }else{
      const val=bg.querySelector("#addRowInput")?.value.trim();
      if(val){const pf=primaryRowField(r);if(pf)pv.values[`${scope}${idx}#${pf}`]=val;}
    }
    close();renderPreview();
  };
  bg.querySelector("#addRowConfirm").addEventListener("click",confirmAdd);
  bg.querySelectorAll("input").forEach(inp=>inp.addEventListener("keydown",e=>{if(e.key==="Enter"){e.preventDefault();confirmAdd();}if(e.key==="Escape")close();}));
}
/* hostRp is the prefix of the row this roster sits in — "" at the top level, or
   "art#0#" for a nested one, so a nested roster deletes its own rows rather than
   nothing at all. Mirrors deleteRow() in public.html. */
function delRow(rname,hostRp,idx){
  const r=allNodes().find(x=>x.kind==="roster"&&x.name===rname);if(!r)return;
  const prefix=rosterScopePrefix(r,hostRp||"");
  const key=prefix+"count";const count=pv.values[key]||1;const nv={};
  Object.keys(pv.values).forEach(k=>{
    if(k===key)return;
    if(k.startsWith(prefix)){const m=k.slice(prefix.length).match(/^(\d+)#(.*)$/);if(m){let ri=+m[1];const fn=m[2];if(ri===idx)return;if(ri>idx)ri--;nv[`${prefix}${ri}#${fn}`]=pv.values[k];return;}}
    nv[k]=pv.values[k];
  });
  nv[key]=Math.max(0,count-1);pv.values=nv;
}
function addRowLabel(r){return r.rowTitle?`+ Add ${esc(r.rowTitle)}`:"+ Add row";}
// Interpolate other fields from the same roster row into label/hint/itemLabel text.
// Supports two syntaxes: {{name}} and ${name} (both are used in various places).
const INTERP_RE=/\{\{\s*([^}]+?)\s*\}\}|\$\{\s*([^}]+?)\s*\}/g;
function rowPrefixes(row){
  if(!row)return [];
  const cur=(row.r!=null&&row.i!=null)?`${row.r}#${row.i}#`:"";
  const parents=Array.isArray(row.parents)?row.parents:[];
  return cur?[cur,...parents]:parents;
}
function rowStoragePrefixes(row){
  if(!row)return [];
  const parents=Array.isArray(row.parents)?[...row.parents].reverse():[];
  const cur=(row.r!=null&&row.i!=null)?`${row.r}#${row.i}#`:"";
  return cur?[...parents,cur]:parents;
}
function rowStoragePrefix(row){return rowStoragePrefixes(row).join("");}
function rosterScopePrefix(r,rowPrefix){return `${rowPrefix||""}${r.name}#`;}
function rosterCountKey(r,rowPrefix){return `${rosterScopePrefix(r,rowPrefix)}count`;}
function rosterRowPrefix(r,rowIdx,rowPrefix){return `${rosterScopePrefix(r,rowPrefix)}${rowIdx}#`;}
function resolveRowValue(name,row){
  const n=String(name||"").trim();
  if(!n)return undefined;
  if(n==="index"&&row&&Number.isFinite(Number(row.i)))return Number(row.i)+1;
  for(const p of rowPrefixes(row)){
    const k=p+n;
    if(k in pv.values)return pv.values[k];
  }
  return undefined;
}
function rowInterp(text,row){
  if(!text)return text;
  return text.replace(INTERP_RE,(_,a,b)=>{
    const n=(a||b).trim();
    const v=resolveRowValue(n,row);
    return v!=null?String(v):`{{${n}}}`;
  });
}
function rowLabel(r,i,row){
  if(r.itemLabel){
    const scope={r:r.name,i,parents:rowPrefixes(row)};
    return esc(r.itemLabel.replace(/\{\{\s*index\s*\}\}|\$\{\s*index\s*\}/g,String(i+1)).replace(INTERP_RE,(_,a,b)=>{const v=resolveRowValue((a||b).trim(),scope);return v!=null?String(v):"";}));
  }
  return r.rowTitle?`${esc(r.rowTitle)} #${i+1}`:`Row #${i+1}`;
}
function pvRoster(r,row){
  const rp=rowStoragePrefix(row);
  const parentPrefixes=rowPrefixes(row);
  const from=String(r.countFrom||"").trim();
  const titleTxt=rowInterp(r.title||r.name,row);
  const count=rosterCount(r,rp);const manual=!r.countFrom;
  /* Carries the prefix of the row this roster sits in. A roster nested inside another
     (Roster → Section → Roster) stores under "art#0#usaha#…", so the bare name made
     every nested roster's buttons identical across the outer rows and the delete
     matched nothing. Mirrors rosterHtml() in public.html. */
  const hostRp=esc(rp||"");
  const canAdd=manual&&!(r.max!==""&&r.max!=null&&count>=Number(r.max));
  ensureRosterDefaultValues(r,count);
  if(r.rosterType==="separate"){
    let h=`<div class="pv-roster" id="pvroster_${esc(r.name)}"><div class="pv-rh">${esc(titleTxt)}<span class="pv-tag">subhalaman</span></div>${rosterValidationMsgs(r,rp)}`;
    if(from&&count<=0){h+=`<div class="pv-rowempty">Fill in “${esc(labelOfField(from))}” first to determine the number of rows.</div>`;}
    else if(count<=0){h+=`<div class="pv-rowempty">No rows yet.</div>`;}
    else{h+=`<div class="pv-rowlist">`;
      for(let i=0;i<count;i++){const sum=rowSummary(r,i);
        h+=`<div class="pv-rowitem"><div class="pv-rowinfo"><b>${rowLabel(r,i,row)}</b><span>${sum||"<i>not filled in</i>"}</span></div>${manual?`<button class="pv-rowdel" data-delrow="${esc(r.name)}" data-rp="${hostRp}" data-i="${i}">remove</button>`:""}<button class="pv-rowopen" data-openrow="${r.uid}" data-i="${i}">${isRowFilled(r,i)?"Edit":"Fill"} →</button></div>`;}
      h+=`</div>`;}
    if(canAdd)h+=`<button class="pv-add" data-addrow="${esc(r.uid)}" data-rp="${hostRp}">${addRowLabel(r)}</button>`;
    return h+`</div>`;
  }
  // inline
  let h=`<div class="pv-roster" id="pvroster_${esc(r.name)}"><div class="pv-rh">${esc(titleTxt)}</div>${rosterValidationMsgs(r,rp)}`;
  if(from&&count<=0)h+=`<div class="pv-rowempty">Fill in “${esc(labelOfField(from))}” first to determine the number of rows.</div>`;
  {const raw=rosterCountRaw(r,rp);if(raw>ROSTER_MAX_ROWS)h+=`<div class="pv-rowempty" style="border-left:3px solid #f59e0b;background:#fffbeb;color:#78350f">Showing the first ${ROSTER_MAX_ROWS} of ${raw} rows. Check the answer that sets the row count — a number this large is usually a typo.</div>`;}
  for(let i=0;i<count;i++){const childRp=rosterRowPrefix(r,i,rp);const rowKey=childRp.slice(0,-1);const rs=computeRosterRowSkipState(r,i);ROW_SKIP_HIDDEN.set(rowKey,rs);rs.forEach(n=>{delete pv.values[`${childRp}${n}`];});h+=`<div class="pv-row"><div class="pv-rownum"><span>${rowLabel(r,i,row)}</span>${manual?`<button class="pv-del" data-delrow="${esc(r.name)}" data-rp="${hostRp}" data-i="${i}">remove</button>`:""}</div>`;r.components.forEach(f=>h+=pvNode(f,{r:r.name,i,parents:parentPrefixes}));h+=`</div>`;}
  if(canAdd)h+=`<button class="pv-add" data-addrow="${esc(r.uid)}" data-rp="${hostRp}">${addRowLabel(r)}</button>`;
  return h+`</div>`;
}
function renderRosterRowPage(){
  const body=document.getElementById("pvBody");const keep=body.scrollTop;
  const r=findNode(pv.row.uid);if(!r){pv.row=null;return renderPreview();}
  const i=pv.row.index;const parent=pageOf(r.uid);
  const rs=computeRosterRowSkipState(r,i);ROW_SKIP_HIDDEN.set(`${r.name}#${i}`,rs);rs.forEach(n=>{delete pv.values[`${r.name}#${i}#${n}`];});
  const pageTitle=esc(parent?(parent.title||parent.name):"Back");
  const rosterTitle=esc(rowInterp(r.title||r.name,{r:r.name,i}));
  const total=rosterCount(r);
  const hasStructure=(r.components||[]).some(c=>c.kind==="block"||c.kind==="section");
  let fieldsHtml=r.components.map(f=>pvNode(f,{r:r.name,i})).join("");
  if(!hasStructure&&fieldsHtml)fieldsHtml=`<div class="pv-card">${fieldsHtml}</div>`;
  let h=`<div class="pv-page"><div class="pv-rrow-hdr"><div class="pv-rrow-bread"><button id="pvBack" class="pv-rrow-back">← ${pageTitle}</button><span class="pv-rrow-sep">›</span><span class="pv-rrow-rname">${rosterTitle}</span><span class="pv-rrow-num">Row ${i+1} of ${total}</span></div><div class="pv-rrow-title">${rowLabel(r,i)}</div></div>${fieldsHtml}</div>`;
  body.innerHTML=h;
  const navArea=document.getElementById("pvNavArea");
  if(navArea)navArea.innerHTML=`<div class="pv-nav"><span></span><button class="btn primary" id="pvBackDone">Save &amp; back</button></div>`;
  bindPreview(body);body.scrollTop=keep;
  document.getElementById("pvBack")?.addEventListener("click",backFromRow);
  document.getElementById("pvBackDone")?.addEventListener("click",backFromRow);
}
function backFromRow(){
  const r=findNode(pv.row.uid);const parent=r?pageOf(r.uid):null;const rname=r?r.name:"";pv.row=null;
  if(parent&&pv.mode==="section"){const idx=visiblePages().indexOf(parent);if(idx>=0)pv.page=idx;}
  renderPreview();setTimeout(()=>{const el=document.getElementById("pvroster_"+rname);if(el)el.scrollIntoView({block:"center"});},40);
}
function pvField(c,row){
  if(c.type==="hidden")return "";
  const rp=rowStoragePrefix(row);
  if(!evalVisible(c.visibleWhen,rp))return "";
  if(row?ROW_SKIP_HIDDEN.get(rp.slice(0,-1))?.has(c.name):SKIP_HIDDEN.has(c.name))return ""; // skip-to: a page or a roster row
  if(c.type==="note")return `<div class="pv-note pv-field">${c.html||""}</div>`;
  if(c.type==="markdown")return `<div class="pv-note pv-field pv-md">${mdToHtml(c.markdown||"")}</div>`;
  const key=row?`${rp}${c.name}`:c.name;
  if(c.type==="calculated"){
    if(c.autofill){
      if(pvEmpty(pv.values[key])||calcPoisoned(pv.values[key])){
        const r=evalExprSrc(c.calculate,rp);
        if(calcUsable(r))pv.values[key]=String(r);
        else delete pv.values[key];   // stay empty rather than record a non-number
      }
      const val=pv.values[key]??"";const isReq=!!c.required||!!(c.requiredWhen&&evalVisible(c.requiredWhen,rp));const en=evalVisible(c.enableWhen,rp);
      const lab=`<label class="pv-lab">${esc(rowInterp(c.label||c.name,row))}${isReq?' <span class="pv-req">*</span>':''}</label>`;const hint=c.hint?`<div class="pv-hint">${esc(rowInterp(c.hint,row))}</div>`:"";
      return `<div class="pv-field" data-fieldkey="${esc(key)}">${lab}${hint}<div><input data-k="${esc(key)}" class="pv-in" value="${esc(val)}"${en?"":" disabled"}></div></div>`;
    }
    const r=evalExprSrc(c.calculate,rp);const ok=calcUsable(r);pv.values[key]=ok?r:"";
    const lab=`<label class="pv-lab">${esc(rowInterp(c.label||c.name,row))}</label>`;const hint=c.hint?`<div class="pv-hint">${esc(rowInterp(c.hint,row))}</div>`:"";
    return `<div class="pv-field" data-fieldkey="${esc(key)}">${lab}${hint}<div><input class="pv-in" value="${esc(ok?String(r):"—")}" disabled></div></div>`;
  }
  if(pvEmpty(pv.values[key])&&clean(c.defaultValue))pv.values[key]=c.defaultValue;
  const val=pv.values[key]??"";
  const isRequired=!!c.required||!!(c.requiredWhen&&evalVisible(c.requiredWhen,rp));
  const enabled=evalVisible(c.enableWhen,rp);
  const dis=enabled?"":" disabled";
  const rdOnly=c.readOnly?" readonly":"";
  const disAll=(enabled&&!c.readOnly)?"":" disabled";
  const lab=`<label class="pv-lab">${esc(rowInterp(c.label||c.name,row))}${isRequired?' <span class="pv-req">*</span>':''}</label>`;
  const hint=c.hint?`<div class="pv-hint">${esc(rowInterp(c.hint,row))}</div>`:"";
  const da=`data-k="${esc(key)}"`;let ctrl="";
  if(c.type==="textarea")ctrl=`<textarea ${da} class="pv-in" rows="3"${dis}${rdOnly}>${esc(val)}</textarea>`;
  else if(c.type==="text")ctrl=`<input ${da} class="pv-in" value="${esc(val)}" placeholder="${esc(c.placeholder||"")}"${dis}${rdOnly}>`;
  else if(c.type==="email")ctrl=`<input ${da} type="email" class="pv-in" value="${esc(val)}" placeholder="${esc(c.placeholder||"name@domain.com")}"${dis}${rdOnly}>`;
  else if(NUMERIC.has(c.type))ctrl=`<input ${da} type="number" class="pv-in" value="${esc(val)}"${c.min!==""&&c.min!=null?` min="${c.min}"`:""}${c.max!==""&&c.max!=null?` max="${c.max}"`:""}${dis}${rdOnly} style="width:auto;min-width:160px">${c.unit?`<span class="pv-unit">${esc(c.unit)}</span>`:""}`;
  else if(c.type==="boolean")ctrl=pvRadios(key,[{value:"true",label:"Yes"},{value:"false",label:"No"}],String(val),disAll);
  else if(c.type==="radio"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>pvRadios(key,ro.opts,String(val),disAll),key);}
  else if(c.type==="select"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>`<select ${da} class="pv-in"${disAll}><option value="">— select —</option>${ro.opts.map(o=>`<option value="${esc(o.value)}"${String(val)===String(o.value)?" selected":""}>${esc(o.label)}</option>`).join("")}</select>`,key);}
  else if(c.type==="checkbox"||c.type==="multiselect"){const ro=resolveOptions(c,rp);ctrl=optWrap(ro,()=>`<div class="pv-radios">${ro.opts.map(o=>`<label class="pv-opt"><input type="checkbox" data-kc="${esc(key)}" value="${esc(o.value)}"${(Array.isArray(val)&&val.map(String).includes(String(o.value)))?" checked":""}${disAll}> ${esc(o.label)}</label>`).join("")}</div>`,key);}
  else if(DATETIME.has(c.type))ctrl=`<input ${da} type="${DT_INPUT_TYPE[c.type]}" class="pv-in" value="${esc(val)}"${clean(c.min)?` min="${esc(c.min)}"`:""}${clean(c.max)?` max="${esc(c.max)}"`:""} style="width:auto"${dis}${rdOnly}>`;
  else if(c.type==="geopoint")ctrl=`<div class="pv-inbtn"><input ${da} class="pv-in" value="${esc(val)}" placeholder="lat, lng"${dis}${rdOnly}><button type="button" class="pv-smbtn pv-geobtn"${disAll}>📍 Get Location</button></div><div class="pv-geomsg"></div>`;
  else if(c.type==="photo"||c.type==="file"){
    const isPhoto=c.type==="photo";
    const disClass=(enabled&&!c.readOnly)?"":" pv-photolabel-dis";
    ctrl=`<div class="pv-photowrap" data-field-type="${isPhoto?'photo':'file'}" data-max-kb="${isPhoto?photoMaxKB(c):0}"><input type="hidden" ${da} value="${esc(val)}"><label class="pv-photolabel${disClass}"><input type="file" class="pv-photofile"${isPhoto?' accept="image/*" capture="environment"':''}${disAll}>${isPhoto?'📷 Take / Choose Photo':'📎 Choose File'}</label><div class="pv-photopreview" hidden></div></div>`;
  }
  else if(c.type==="signature")ctrl=`<div class="pv-sigwrap"><input type="hidden" ${da}><canvas class="pv-sigpad" width="400" height="140"${(enabled&&!c.readOnly)?"":' data-disabled="1"'}></canvas><div class="pv-sigactions"><button type="button" class="pv-smbtn pv-sigclear"${disAll}>Clear signature</button></div></div>`;
  else if(c.type==="barcode")ctrl=`<div class="pv-inbtn"><input ${da} class="pv-in" value="${esc(val)}" placeholder="scan / type code"${dis}${rdOnly}><button type="button" class="pv-smbtn pv-scanbtn"${disAll}>📷 Pindai</button></div>`;
  else ctrl=`<input ${da} class="pv-in" value="${esc(val)}"${dis}${rdOnly}>`;
  let vmsg="";
  (c.validations||[]).forEach(v=>{if(!v.test)return;if(pvEmpty(val))return;const r=evalExprSrc(v.test,rp);if(r===undefined)return;if(!r)vmsg+=`<div class="pv-vmsg ${v.severity==="warning"?"warning":"error"}">${esc(textOf(v.message)||"Invalid value")}</div>`;});
  return `<div class="pv-field" data-fieldkey="${esc(key)}">${lab}${hint}<div>${ctrl}</div>${vmsg}</div>`;
}
function snapScroll(fkey,top0){
  if(!fkey||top0==null)return;
  const pvBody=document.getElementById("pvBody");if(!pvBody)return;
  const nf=pvBody.querySelector(`.pv-field[data-fieldkey="${CSS.escape(fkey)}"]`);
  if(!nf)return;
  const delta=Math.round(nf.getBoundingClientRect().top-top0);
  if(delta)pvBody.scrollTop+=delta;
}
function pvRadios(key,opts,val,dis){dis=dis||"";return `<div class="pv-radios">${opts.map(o=>`<label class="pv-opt"><input type="radio" name="r_${esc(key)}" data-kr="${esc(key)}" value="${esc(o.value)}"${String(val)===String(o.value)?" checked":""}${dis}> ${esc(o.label)}</label>`).join("")}</div>`;}
function fieldNameFromKey(key){const p=String(key||"").split("#");return p[p.length-1]||"";}
function isCountSourceFieldKey(key){
  const f=fieldNameFromKey(key);
  if(!f)return false;
  return allNodes().some(n=>n.kind==="roster"&&String(n.countFrom||"").trim()===f);
}
function bindPreview(body){
  body.querySelectorAll("[data-k]").forEach(inp=>{
    const key=inp.getAttribute("data-k");
    inp.addEventListener("input",e=>{
      pv.values[key]=inp.value;
      if(isCountSourceFieldKey(key)){
        const f=e.target.closest(".pv-field"),fkey=f?.dataset?.fieldkey,top0=fkey?f.getBoundingClientRect().top:null;
        renderPreview();
        snapScroll(fkey,top0);
      }
    });
    inp.addEventListener("change",e=>{pv.values[key]=inp.value;const f=e.target.closest(".pv-field"),fkey=f?.dataset?.fieldkey,top0=fkey?f.getBoundingClientRect().top:null;renderPreview();snapScroll(fkey,top0);});
  });
  body.querySelectorAll("[data-kr]").forEach(inp=>inp.addEventListener("change",e=>{pv.values[inp.getAttribute("data-kr")]=inp.value;const f=e.target.closest(".pv-field"),fkey=f?.dataset?.fieldkey,top0=fkey?f.getBoundingClientRect().top:null;renderPreview();snapScroll(fkey,top0);}));
  body.querySelectorAll("[data-kc]").forEach(inp=>inp.addEventListener("change",e=>{const key=inp.getAttribute("data-kc");const arr=Array.isArray(pv.values[key])?pv.values[key]:[];const v=inp.value;if(inp.checked){if(!arr.includes(v))arr.push(v);}else{const idx=arr.indexOf(v);if(idx>=0)arr.splice(idx,1);}pv.values[key]=arr;const f=e.target.closest(".pv-field"),fkey=f?.dataset?.fieldkey,top0=fkey?f.getBoundingClientRect().top:null;renderPreview();snapScroll(fkey,top0);}));
  body.querySelectorAll("[data-addrow]").forEach(b=>b.addEventListener("click",()=>{const r=findNode(b.getAttribute("data-addrow"));if(r)openAddRowModal(r,b.getAttribute("data-rp")||"");}));
  body.querySelectorAll("[data-delrow]").forEach(b=>b.addEventListener("click",()=>{delRow(b.getAttribute("data-delrow"),b.getAttribute("data-rp")||"",+b.getAttribute("data-i"));renderPreview();}));
  body.querySelectorAll("[data-openrow]").forEach(b=>b.addEventListener("click",()=>{pv.row={uid:b.getAttribute("data-openrow"),index:+b.getAttribute("data-i")};document.getElementById("pvBody").scrollTop=0;renderPreview();}));
  body.querySelectorAll(".pv-sigpad").forEach(canvas=>wireSignaturePad(canvas));
  body.querySelectorAll(".pv-geobtn").forEach(btn=>wireGeoButton(btn));
  body.querySelectorAll(".pv-scanbtn").forEach(btn=>wireScanButton(btn));
  body.querySelectorAll(".pv-photowrap").forEach(wrap=>wirePhotoField(wrap));
}

/* ---- widget field khusus: tanda tangan, lokasi, pindai barcode (preview) ---- */
function wireSignaturePad(canvas){
  if(canvas._wired)return;canvas._wired=true;
  const hidden=canvas.previousElementSibling;
  if(!hidden)return;
  const ctx=canvas.getContext("2d");
  ctx.lineWidth=2.2;ctx.lineCap="round";ctx.lineJoin="round";ctx.strokeStyle="#13171e";
  if(hidden.value)loadSignatureImage(canvas,hidden.value);
  let drawing=false,last=null;
  const pos=e=>{const r=canvas.getBoundingClientRect();return {x:(e.clientX-r.left)*(canvas.width/r.width),y:(e.clientY-r.top)*(canvas.height/r.height)};};
  const start=e=>{if(canvas.dataset.disabled==="1")return;e.preventDefault();drawing=true;last=pos(e);};
  const move=e=>{if(!drawing)return;e.preventDefault();const p=pos(e);ctx.beginPath();ctx.moveTo(last.x,last.y);ctx.lineTo(p.x,p.y);ctx.stroke();last=p;};
  const end=()=>{
    if(!drawing)return;drawing=false;
    hidden.value=canvas.toDataURL("image/png");
    hidden.dispatchEvent(new Event("change",{bubbles:true}));
  };
  canvas.addEventListener("pointerdown",start);
  canvas.addEventListener("pointermove",move);
  canvas.addEventListener("pointerup",end);
  canvas.addEventListener("pointercancel",end);
  canvas.addEventListener("pointerleave",end);
  const clearBtn=canvas.parentElement.querySelector(".pv-sigclear");
  clearBtn?.addEventListener("click",()=>{
    ctx.clearRect(0,0,canvas.width,canvas.height);
    hidden.value="";
    hidden.dispatchEvent(new Event("change",{bubbles:true}));
  });
}
function loadSignatureImage(canvas,dataUrl){
  const ctx=canvas.getContext("2d");
  const img=new Image();
  img.onload=()=>{ctx.clearRect(0,0,canvas.width,canvas.height);ctx.drawImage(img,0,0,canvas.width,canvas.height);};
  img.src=dataUrl;
}
function wireGeoButton(btn){
  if(btn._wired)return;btn._wired=true;
  btn.addEventListener("click",()=>{
    const wrap=btn.closest(".pv-field");
    const msg=wrap?.querySelector(".pv-geomsg");
    const input=btn.previousElementSibling;
    if(msg){msg.textContent="";msg.classList.remove("error");}
    if(!navigator.geolocation){if(msg){msg.textContent="Geolocation is not supported by this browser.";msg.classList.add("error");}return;}
    const orig=btn.textContent;btn.disabled=true;btn.textContent="Searching…";
    navigator.geolocation.getCurrentPosition(
      pos=>{
        btn.disabled=false;btn.textContent=orig;
        input.value=`${pos.coords.latitude.toFixed(6)}, ${pos.coords.longitude.toFixed(6)}`;
        input.dispatchEvent(new Event("change",{bubbles:true}));
      },
      err=>{
        btn.disabled=false;btn.textContent=orig;
        if(msg){msg.textContent=err.code===1?"Location permission denied.":"Failed to get location: "+err.message;msg.classList.add("error");}
      },
      {enableHighAccuracy:true,timeout:15000}
    );
  });
}
function wireScanButton(btn){
  if(btn._wired)return;btn._wired=true;
  if(!("BarcodeDetector" in window)){btn.disabled=true;btn.title="Automatic scanning is not supported by this browser — enter manually.";return;}
  btn.addEventListener("click",()=>openBarcodeScanner(btn.previousElementSibling));
}
async function openBarcodeScanner(input){
  const bg=document.createElement("div");bg.className="pv-modal-bg";
  bg.innerHTML=`<div class="pv-modal pv-scanmodal">
    <h3>Scan Barcode</h3>
    <p class="pv-modal-sub">Point the camera at the barcode/QR code.</p>
    <video id="scanVideo" autoplay playsinline muted></video>
    <div class="pv-modal-actions"><button class="btn ghost" id="scanCancel">Cancel</button></div>
  </div>`;
  document.body.appendChild(bg);
  const video=bg.querySelector("#scanVideo"),sub=bg.querySelector(".pv-modal-sub");
  let stream=null,stopped=false;
  const close=()=>{stopped=true;if(stream)stream.getTracks().forEach(t=>t.stop());bg.remove();};
  bg.querySelector("#scanCancel").addEventListener("click",close);
  bg.addEventListener("click",e=>{if(e.target===bg)close();});
  try{
    stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:"environment"}});
    video.srcObject=stream;
    const detector=new BarcodeDetector();
    const loop=async()=>{
      if(stopped)return;
      try{
        const codes=await detector.detect(video);
        if(codes.length){
          input.value=codes[0].rawValue;
          input.dispatchEvent(new Event("change",{bubbles:true}));
          close();
          return;
        }
      }catch(_){}
      requestAnimationFrame(loop);
    };
    requestAnimationFrame(loop);
  }catch(e){
    if(sub)sub.textContent="Cannot access camera: "+e.message;
  }
}

function wirePhotoField(wrap){
  if(wrap._wired)return;wrap._wired=true;
  const hidden=wrap.querySelector("input[type=hidden]");
  const fileInput=wrap.querySelector(".pv-photofile");
  const preview=wrap.querySelector(".pv-photopreview");
  if(!hidden||!fileInput||!preview)return;
  if(hidden.value)showPhotoPreview(preview,hidden.value,fileInput.disabled);
  fileInput.addEventListener("change",async()=>{
    let f=fileInput.files[0];if(!f)return;
    // The preview never uploads, but running the same compression here is what lets
    // an admin see what their KB setting actually does to a real camera photo.
    const maxKB=Number(wrap.dataset.maxKb||0);
    let note="";
    if(wrap.dataset.fieldType==="photo"&&maxKB>0&&window.ImageCompress){
      const res=await ImageCompress.compress(f,maxKB);
      if(res.changed){
        f=res.file;
        note=`${fmtBytes(res.originalSize)} → ${fmtBytes(f.size)}`+(res.reachedTarget?"":` (still above ${maxKB} KB)`);
      }else{
        note=`${fmtBytes(f.size)} — ${res.reason}`;
      }
    }
    const reader=new FileReader();
    reader.onload=e=>{
      hidden.value=e.target.result;
      hidden.dispatchEvent(new Event("change",{bubbles:true}));
      showPhotoPreview(preview,e.target.result,false,note);
    };
    reader.readAsDataURL(f);
  });
  preview.addEventListener("click",e=>{
    if(!e.target.classList.contains("pv-photoclear"))return;
    hidden.value="";fileInput.value="";
    preview.hidden=true;preview.innerHTML="";
    hidden.dispatchEvent(new Event("change",{bubbles:true}));
  });
}
/* Defined in /image-compress.js — see the note on the same helper in public.html. */
function photoMaxKB(c){return window.ImageCompress?ImageCompress.maxKBFor(c):0;}
function fmtBytes(b){const n=Number(b)||0;return n>=1048576?(n/1048576).toFixed(1)+" MB":Math.max(1,Math.round(n/1024))+" KB";}
function showPhotoPreview(preview,dataUrl,disabled,note){
  const isImage=dataUrl.startsWith("data:image");
  const dis=disabled?" disabled":"";
  const sizeNote=note?`<div style="flex-basis:100%;font-size:11.5px;color:var(--muted)">${esc(note)}</div>`:"";
  if(isImage)
    preview.innerHTML=`<img class="pv-photoimg" src="${dataUrl}"><button type="button" class="pv-smbtn pv-photoclear"${dis}>Remove photo</button>${sizeNote}`;
  else
    preview.innerHTML=`<span style="font-size:12.5px;color:var(--ink-soft)">File selected</span><button type="button" class="pv-smbtn pv-photoclear"${dis}>Delete</button>${sizeNote}`;
  preview.hidden=false;
}

function renderPvSide(){
  const side=document.getElementById("pvSide");if(!side)return;
  const pages=visiblePages();
  let activeIdx=-1,activeRosterUid=null;
  if(pv.row){const rn=findNode(pv.row.uid);activeRosterUid=pv.row.uid;const pp=rn&&pageOf(rn.uid);activeIdx=pp?pages.indexOf(pp):-1;}
  else if(pv.mode==="section")activeIdx=pv.page;
  let html=`<div class="pv-side-h">Page list</div>`;
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

/* ===================== BOOT ===================== */
buildPalette();
view={type:"page",uid:state.pages[0].uid};
render();

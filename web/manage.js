/* The single-form management page (Overview/Share/Viewer Access/Editor Access).
   Self-contained like the other pages in this app (builder-bridge.js, admin.js) — the basic
   helpers (api/esc/adminToast/adminConfirm) are deliberately duplicated, not imported. */
const token=localStorage.getItem("eform_token");
if(!token) location.replace("/login");
const H={"Authorization":"Bearer "+token,"Content-Type":"application/json"};
const $=s=>document.querySelector(s);
const esc=s=>String(s??"").replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));

const FORM_ID=new URLSearchParams(location.search).get("id");
if(!FORM_ID) location.replace("/admin");

let _toastTimer=null;
function adminToast(msg,isErr){
  let el=document.getElementById("adminToast");
  if(!el){el=document.createElement("div");el.id="adminToast";el.style.cssText="position:fixed;bottom:24px;right:24px;z-index:9999;padding:10px 18px;border-radius:8px;font-size:13px;font-weight:600;box-shadow:0 4px 20px rgba(0,0,0,.18);transition:.2s;opacity:0;pointer-events:none";document.body.appendChild(el);}
  el.textContent=msg;
  el.style.background=isErr?"#b91c1c":"#15803d";
  el.style.color="#fff";
  el.style.opacity="1";
  clearTimeout(_toastTimer);
  _toastTimer=setTimeout(()=>{el.style.opacity="0";},3000);
}

function adminConfirm(msg,onConfirm){
  const dlg=document.getElementById("confirmDlg");
  document.getElementById("confirmMsg").textContent=msg;
  const yes=document.getElementById("confirmYes");
  const no=document.getElementById("confirmNo");
  const cleanup=()=>{dlg.close();yes.onclick=null;no.onclick=null;};
  yes.onclick=()=>{cleanup();onConfirm();};
  no.onclick=cleanup;
  dlg.showModal();
}

async function api(path,opts={}){
  const r=await fetch(path,{...opts,headers:{...H,...(opts.headers||{})}});
  if(r.status===401){localStorage.removeItem("eform_token");location.replace("/login");throw new Error("session expired");}
  const ct=r.headers.get("content-type")||""; const data=ct.includes("json")?await r.json():null;
  if(!r.ok) throw new Error((data&&data.error)||("HTTP "+r.status));
  return data;
}

let MY_ROLE="admin";
let FORM_SCHEMA=null;
let FORM_TITLE="";
let FORM_STATUS="draft";

(async()=>{
  try{
    const me=await api("/api/auth/me");
    MY_ROLE=me.role||"admin";
    const uname=me.username||"",urole=me.role||"";
    const av=$("#userAvatar");if(av)av.textContent=uname.charAt(0).toUpperCase()||"?";
    const un=$("#userName");if(un)un.textContent=uname;
    const ur=$("#userRole");if(ur)ur.textContent=urole;
    const dn=$("#uddName");if(dn)dn.textContent=uname;
    const dr=$("#uddRole");if(dr)dr.textContent=urole;
  }catch(e){}
  document.getElementById("navBuilder").href="/builder?id="+FORM_ID;
  document.getElementById("navResponses").href="/responses?id="+FORM_ID;
  await loadForm();
  const wantedSection=new URLSearchParams(location.search).get("section");
  if(["share","user","api"].includes(wantedSection)) switchSection(wantedSection);
})();

(function(){
  const userBtn=document.getElementById("userBtn");
  const dropdown=document.getElementById("userDropdown");
  if(userBtn&&dropdown){
    userBtn.addEventListener("click",e=>{e.stopPropagation();dropdown.hidden=!dropdown.hidden;});
    document.addEventListener("click",e=>{if(!dropdown.hidden&&!dropdown.contains(e.target))dropdown.hidden=true;});
  }
})();

$("#logout").addEventListener("click",()=>{localStorage.removeItem("eform_token");localStorage.removeItem("eform_user");location.replace("/login");});

/* ======================================================
   RINGKASAN
   ====================================================== */

async function loadForm(){
  try{
    const f=await api("/api/forms/"+FORM_ID);
    FORM_SCHEMA=f.schema; FORM_TITLE=f.title; FORM_STATUS=f.status;
    document.getElementById("mgFormTitle").textContent=f.title+" · eForm";
    document.getElementById("mgSidebarTitle").textContent=f.title;
    renderOverview(f);
  }catch(e){
    adminToast("Failed to load form: "+e.message,true);
  }
}

async function renderOverview(f){
  const statusEl=document.getElementById("ovStatus");
  statusEl.textContent=f.status;
  statusEl.className="tag "+f.status;
  document.getElementById("ovUpdated").textContent="Updated "+new Date(f.updatedAt).toLocaleString("id-ID");
  const pubBtn=document.getElementById("ovPubBtn");
  pubBtn.textContent=f.status==="published"?"Unpublish":"Publish";
  const respEl=document.getElementById("ovResponses");
  const delBtn=document.getElementById("ovDeleteBtn");
  respEl.textContent="Loading response count…";
  try{
    const d=await api("/api/forms/"+FORM_ID+"/responses?limit=1");
    respEl.textContent=d.total+" responses";
    delBtn.disabled=d.total>0;
    delBtn.title=d.total>0?"Cannot be deleted because it already has responses":"";
  }catch(e){respEl.textContent="";}
}

async function togglePub(){
  const next=FORM_STATUS==="published"?"draft":"published";
  try{await api("/api/forms/"+FORM_ID+"/publish",{method:"POST",body:JSON.stringify({status:next})});await loadForm();}
  catch(e){adminToast(e.message,true);}
}

function delForm(){
  adminConfirm("Delete form \""+FORM_TITLE+"\"? A form can only be deleted while it has no responses.",async()=>{
    try{await api("/api/forms/"+FORM_ID,{method:"DELETE"});location.href="/admin";}catch(e){adminToast(e.message,true);}
  });
}

/* ======================================================
   SIDEBAR — pindah antar section (lazy-load data di kunjungan pertama)
   ====================================================== */

let _shareInited=false,_userInited=false,_apiInited=false;
function switchSection(sec){
  ["overview","share","user","api"].forEach(s=>{
    const el=document.getElementById("sec-"+s);
    const btn=document.getElementById("nav-"+s);
    if(el) el.hidden=s!==sec;
    if(btn) btn.classList.toggle("active",s===sec);
  });
  const shown=document.getElementById("sec-"+sec);
  if(shown){shown.classList.add("fade-in");setTimeout(()=>shown.classList.remove("fade-in"),200);}
  if(sec==="overview") loadForm();
  else if(sec==="share"&&!_shareInited){_shareInited=true;initShareSection();}
  else if(sec==="user"&&!_userInited){_userInited=true;initUserSection();}
  else if(sec==="api"&&!_apiInited){_apiInited=true;initApiSection();}

  const params=new URLSearchParams(location.search);
  if(sec==="overview") params.delete("section"); else params.set("section",sec);
  const qs=params.toString();
  history.replaceState(null,"",location.pathname+(qs?"?"+qs:""));
}

/* ======================================================
   SHARE (formerly shareDlg)
   ====================================================== */

function initShareSection(){
  refreshShares();
}

function openShareCreateDlg(){
  $("#shareNote").innerHTML = FORM_STATUS==="published"
    ? "The form is already <b>published</b> — the link can be accessed publicly right away."
    : "⚠️ The form is still <b>draft</b>. The link has been created, but the public can only open it once it's published.";
  $("#shareLabel").value="";$("#sharePw").value="";
  $("#shareMulti").checked=false;$("#shareAllow").checked=true;
  document.getElementById("shareAccessPublic").checked=true;
  $("#restrictedSection").style.display="none";
  pendingEmails=[];renderPendingEmails();
  shareCreateDlg.showModal();
}

document.getElementById("shareAccessRestricted").addEventListener("change",()=>{
  $("#restrictedSection").style.display="block";
  $("#newEmailInput").focus();
});
document.getElementById("shareAccessPublic").addEventListener("change",()=>{
  $("#restrictedSection").style.display="none";
});

let pendingEmails=[];
function renderPendingEmails(){
  $("#newEmailList").innerHTML=pendingEmails.length
    ?`<table class="email-tbl"><tbody>${pendingEmails.map((e,i)=>`<tr>
        <td>${esc(e.email)}</td>
        <td class="muted">${esc(e.note)}</td>
        <td><button class="btn danger btn-xs" onclick="removePending(${i})">✕</button></td>
      </tr>`).join("")}</tbody></table>`
    :"<div class='muted' style='font-size:12px;padding:4px 0'>No emails added yet.</div>";
}
function removePending(i){pendingEmails.splice(i,1);renderPendingEmails();}
$("#btnAddNewEmail").addEventListener("click",()=>{
  const email=$("#newEmailInput").value.trim().toLowerCase();
  const note=$("#newEmailNote").value.trim();
  if(!email){$("#newEmailInput").focus();return;}
  if(pendingEmails.some(e=>e.email===email)){adminToast("Email is already in the list",true);return;}
  pendingEmails.push({email,note});
  $("#newEmailInput").value="";$("#newEmailNote").value="";
  $("#newEmailInput").focus();
  renderPendingEmails();
});
$("#newEmailInput").addEventListener("keydown",e=>{if(e.key==="Enter"){e.preventDefault();$("#btnAddNewEmail").click();}});

function toLocalDT(iso){
  if(!iso)return"";
  const d=new Date(iso);
  const p=n=>String(n).padStart(2,"0");
  return`${d.getFullYear()}-${p(d.getMonth()+1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

let editingShareId=null;
function startEdit(id){editingShareId=id;refreshShares();}
function cancelEdit(){editingShareId=null;refreshShares();}

async function saveShareEdit(id,hasPassword){
  const label=(document.getElementById("elabel_"+id)?.value||"").trim();
  const allowResponses=document.getElementById("eallow_"+id)?.checked??true;
  const multiResponse=document.getElementById("emulti_"+id)?.checked??false;
  const accessMode=document.querySelector(`input[name="eacc_${id}"]:checked`)?.value||"public";
  const pwInput=(document.getElementById("epw_"+id)?.value||"");
  const clearPw=document.getElementById("eclearpw_"+id)?.checked||false;
  const updatePassword=pwInput!==""||clearPw;
  const password=clearPw?"":pwInput;
  const expInput=(document.getElementById("eexp_"+id)?.value||"");
  const expiresAt=expInput?new Date(expInput).toISOString():"";
  const btn=document.getElementById("esave_"+id);
  if(btn){btn.disabled=true;btn.textContent="Saving…";}
  try{
    await api("/api/shares/"+id,{method:"PATCH",body:JSON.stringify({
      label,allowResponses,multiResponse,accessMode,
      updatePassword,password,
      updateExpiry:true,expiresAt
    })});
    editingShareId=null;refreshShares();
  }catch(e){adminToast(e.message,true);if(btn){btn.disabled=false;btn.textContent="Save";}}
}

const ICON_LOCK='<svg viewBox="0 0 20 20" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4.5" y="9" width="11" height="8" rx="1.5"/><path d="M7 9V6a3 3 0 0 1 6 0v3"/></svg>';
const ICON_COPY='<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="7" y="7" width="10" height="10" rx="1.5"/><path d="M4.5 13H3.5A1.5 1.5 0 0 1 2 11.5v-8A1.5 1.5 0 0 1 3.5 2h8A1.5 1.5 0 0 1 13 3.5v1"/></svg>';
const ICON_EYE_SM='<svg viewBox="0 0 20 20" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M1.5 10 Q10 2 18.5 10 Q10 18 1.5 10 Z"/><circle cx="10" cy="10" r="2.3"/></svg>';

function copyShareUrl(url){
  navigator.clipboard.writeText(url).then(()=>adminToast("Link copied")).catch(()=>adminToast("Copy failed",true));
}

async function refreshShares(){
  try{
    const {shares}=await api("/api/forms/"+FORM_ID+"/shares");
    if(!shares||!shares.length){
      $("#shareList").innerHTML='<div class="share-empty muted">No share links yet. Click "+ Create Share Link" to create the first one.</div>';
      return;
    }
    const emailMap={};
    await Promise.all(shares.filter(s=>s.accessMode==="restricted").map(async s=>{
      try{const {emails}=await api("/api/shares/"+s.id+"/allowed-emails");emailMap[s.id]=emails||[];}catch{emailMap[s.id]=[];}
    }));
    $("#shareList").innerHTML=shares.map(s=>{
      const isEditing=s.id===editingShareId;
      const badges=[];
      if(s.hasPassword)badges.push(`<span class="tag tag-icon">${ICON_LOCK} Password</span>`);
      if(s.multiResponse)badges.push('<span class="tag">Multi-response</span>');
      if(s.accessMode==="restricted")badges.push('<span class="tag">Terbatas</span>');

      const editSection=isEditing?`<div class="share-edit">
        <div class="edit-row"><span class="edit-lbl">Label</span>
          <input id="elabel_${s.id}" value="${esc(s.label||"")}" style="flex:1">
        </div>
        <div class="edit-row" style="gap:16px;flex-wrap:wrap">
          <label class="muted"><input type="checkbox" id="eallow_${s.id}" ${s.allowResponses?"checked":""}> Accepting responses</label>
          <label class="muted"><input type="checkbox" id="emulti_${s.id}" ${s.multiResponse?"checked":""}> Multi-response</label>
        </div>
        <div class="edit-row" style="gap:16px;flex-wrap:wrap">
          <span class="edit-lbl">Akses</span>
          <label class="muted"><input type="radio" name="eacc_${s.id}" value="public" ${s.accessMode!=="restricted"?"checked":""}> Publik</label>
          <label class="muted"><input type="radio" name="eacc_${s.id}" value="restricted" ${s.accessMode==="restricted"?"checked":""}> Terbatas</label>
        </div>
        <div class="edit-row"><span class="edit-lbl">New password</span>
          <input id="epw_${s.id}" type="text" placeholder="${s.hasPassword?"Password already set — fill in to change":"Optional"}" style="flex:1">
        </div>
        ${s.hasPassword?`<div class="edit-row"><span class="edit-lbl"></span>
          <label class="muted"><input type="checkbox" id="eclearpw_${s.id}"> Remove the existing password</label>
        </div>`:""}
        <div class="edit-row"><span class="edit-lbl">Expires</span>
          <input id="eexp_${s.id}" type="datetime-local" value="${toLocalDT(s.expiresAt)}" style="flex:1">
          <span class="muted" style="font-size:11px">Leave empty = no limit</span>
        </div>
        <div class="acts" style="margin-top:10px">
          <button class="btn primary btn-sm" id="esave_${s.id}" onclick="saveShareEdit('${s.id}',${s.hasPassword})">Save</button>
          <button class="btn btn-sm" onclick="cancelEdit()">Cancel</button>
        </div>
      </div>`:"";

      let emailSection="";
      if(s.accessMode==="restricted"&&!isEditing){
        const emails=emailMap[s.id]||[];
        const rows=emails.length
          ?emails.map(e=>`<tr><td>${esc(e.email)}</td><td class="muted">${esc(e.note)}</td><td><button class="btn danger btn-xs" onclick="removeEmail('${e.id}')">✕</button></td></tr>`).join("")
          :`<tr><td colspan="3" class="muted" style="padding:6px 0">No accounts registered yet.</td></tr>`;
        emailSection=`<div class="email-sect">
          <div class="email-sect-h">Allowed accounts (${emails.length})</div>
          <table class="email-tbl"><tbody>${rows}</tbody></table>
          <div class="row" style="gap:6px;margin-top:8px">
            <input id="addIn_${s.id}" type="email" placeholder="email@example.com" style="flex:2">
            <input id="addNote_${s.id}" placeholder="Note" style="flex:2">
            <button class="btn btn-xs" onclick="addEmailToShare('${s.id}')">+ Add</button>
          </div>
        </div>`;
      }

      return `<div class="share-card">
        <div class="share-card-top">
          <div class="share-card-title">
            <b>${esc(s.label||"(no label)")}</b>
            <span class="tag ${s.isActive?"published":"archived"}">${s.isActive?"Active":"Inactive"}</span>
          </div>
          ${s.isActive&&!isEditing?`<button class="btn btn-xs" onclick="startEdit('${s.id}')">Edit</button>`:""}
        </div>
        ${badges.length?`<div class="share-badges">${badges.join("")}</div>`:""}
        <div class="share-url-row">
          <code class="share-url">${esc(s.shareUrl)}</code>
          <button class="share-copy-btn" type="button" title="Copy link" onclick="copyShareUrl('${esc(s.shareUrl)}')">${ICON_COPY}</button>
        </div>
        <div class="share-meta muted">${ICON_EYE_SM} ${s.viewCount}× dibuka</div>
        ${editSection}${emailSection}
        <div class="acts" style="margin-top:10px">
          <a class="btn" href="${esc(s.shareUrl)}" target="_blank">Open</a>
          ${!isEditing?(s.isActive
            ?`<button class="btn danger" onclick="revoke('${s.id}')">Revoke</button>`
            :`<button class="btn" onclick="reactivateShare('${s.id}')">Reactivate</button><button class="btn danger" onclick="deleteShare('${s.id}')">Delete</button>`
          ):""}
        </div>
      </div>`;
    }).join("");
  }catch(e){ $("#shareList").innerHTML=esc(e.message); }
}

async function addEmailToShare(shareId){
  const inEl=document.getElementById("addIn_"+shareId);
  const noteEl=document.getElementById("addNote_"+shareId);
  const email=(inEl?.value||"").trim().toLowerCase();
  const note=(noteEl?.value||"").trim();
  if(!email){inEl?.focus();return;}
  try{
    await api("/api/shares/"+shareId+"/allowed-emails",{method:"POST",body:JSON.stringify({email,note})});
    if(inEl)inEl.value="";if(noteEl)noteEl.value="";
    refreshShares();
  }catch(e){adminToast(e.message,true);}
}
async function removeEmail(id){
  try{await api("/api/share-emails/"+id,{method:"DELETE"});refreshShares();}catch(e){adminToast(e.message,true);}
}
async function revoke(id){ try{await api("/api/shares/"+id,{method:"DELETE"});refreshShares();}catch(e){adminToast(e.message,true);} }
async function reactivateShare(id){
  try{await api("/api/shares/"+id+"/reactivate",{method:"POST"});refreshShares();adminToast("Link reactivated");}
  catch(e){adminToast(e.message,true);}
}
async function deleteShare(id){
  adminConfirm("Permanently delete this link and all its configuration?",async()=>{
    try{await api("/api/shares/"+id+"/permanent",{method:"DELETE"});refreshShares();}catch(e){adminToast(e.message,true);}
  });
}
$("#makeShare").addEventListener("click",async()=>{
  try{
    const accessMode=document.querySelector("input[name='shareAccess']:checked")?.value||"public";
    const sh=await api("/api/forms/"+FORM_ID+"/shares",{method:"POST",body:JSON.stringify({
      label:$("#shareLabel").value.trim(),
      allowResponses:$("#shareAllow").checked,
      multiResponse:$("#shareMulti").checked,
      accessMode,
      password:$("#sharePw").value
    })});
    if(accessMode==="restricted"&&pendingEmails.length){
      await Promise.all(pendingEmails.map(e=>
        api("/api/shares/"+sh.id+"/allowed-emails",{method:"POST",body:JSON.stringify(e)}).catch(()=>{})
      ));
    }
    pendingEmails=[];renderPendingEmails();
    $("#shareLabel").value="";$("#sharePw").value="";$("#shareMulti").checked=false;
    document.getElementById("shareAccessPublic").checked=true;
    $("#restrictedSection").style.display="none";
    shareCreateDlg.close();
    refreshShares();
    adminToast("Share link created");
  }catch(e){adminToast(e.message,true);}
});

/* ======================================================
   EDITOR ACCESS — what the combined User table still uses
   ====================================================== */

let _epPermCache=[];

async function removeEditorPerm(permId,name){
  adminConfirm(`Revoke editor access for "${name}" on this form?`,async()=>{
    try{
      await api("/api/editor-permissions/"+permId,{method:"DELETE"});
      await refreshUserPermList();
    }catch(e){
      adminToast("Failed: "+e.message,true);
    }
  });
}

/* ======================================================
   VIEWER ACCESS — what the combined User table still uses
   ====================================================== */

let _vpdFilters={};
let _epdPermId=null, _epdFilters={};

function renderFilterChips(containerId,filters,removeFn){
  const el=document.getElementById(containerId);
  if(!el)return;
  const entries=Object.entries(filters||{});
  if(!entries.length){
    el.innerHTML='<span style="font-size:11px;color:var(--muted)">No filter restrictions yet.</span>';
    return;
  }
  el.innerHTML=entries.map(([k,v])=>`
    <span style="display:inline-flex;align-items:center;gap:3px;background:var(--surface);border:1px solid var(--line);border-radius:4px;padding:2px 6px;margin:2px;font-size:11px">
      ${esc(k)}: <b>${esc(v)}</b>
      <button onclick="${removeFn}('${esc(k)}')" style="background:none;border:none;cursor:pointer;color:var(--muted);padding:0 2px;line-height:1;font-size:12px">✕</button>
    </span>`).join('');
}

function buildFieldOptions(schema,selectId){
  const sel=document.getElementById(selectId);
  if(!sel)return;
  const fields=[];
  function walk(comps){
    for(const c of comps||[]){
      if(c.kind==="field"&&c.name&&c.type!=="note"&&c.type!=="hidden"&&c.type!=="markdown")
        fields.push({name:c.name,label:typeof c.label==="string"?c.label:(c.label?.id||c.name)});
      else if(c.components)walk(c.components);
    }
  }
  for(const p of schema?.pages||[])walk(p.components||[]);
  const cur=sel.value;
  sel.innerHTML='<option value="">— field —</option>'+
    fields.map(f=>`<option value="${esc(f.name)}">${esc(f.label)}</option>`).join('');
  sel.value=cur;
}

function addVpdFilter(){
  const field=document.getElementById("vpdFilterField").value;
  const value=(document.getElementById("vpdFilterValue").value||"").trim();
  if(!field||!value){adminToast("Select a field and enter a value",true);return;}
  _vpdFilters[field]=value;
  document.getElementById("vpdFilterValue").value="";
  renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");
}
function removeVpdFilter(field){
  delete _vpdFilters[field];
  renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");
}

function addEpdFilter(){
  const field=document.getElementById("epdFilterField").value;
  const value=(document.getElementById("epdFilterValue").value||"").trim();
  if(!field||!value){adminToast("Select a field and enter a value",true);return;}
  _epdFilters[field]=value;
  document.getElementById("epdFilterValue").value="";
  renderFilterChips("epdFilterList",_epdFilters,"removeEpdFilter");
}
function removeEpdFilter(field){
  delete _epdFilters[field];
  renderFilterChips("epdFilterList",_epdFilters,"removeEpdFilter");
}

async function openEpDetail(permId,editorName){
  _epdPermId=permId;
  _epdFilters={};
  document.getElementById("epdEditorName").textContent=editorName;
  // From the row on screen — see openVpDetail.
  document.getElementById("epdNote").value=
    (_epPermCache.find(p=>p.id===permId)||{}).editorNote||"";
  try{
    const [perm,allowedData,respondentsData]=await Promise.all([
      api("/api/editor-permissions/"+permId),
      api("/api/editor-permissions/"+permId+"/respondents").catch(()=>({respondents:[]})),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);

    document.querySelector(`input[name='epdRA'][value='${perm.respondentAccess}']`).checked=true;
    toggleEditorRespondentSection(perm.respondentAccess==="selected");

    _epdFilters=perm.fieldFilters||{};
    buildFieldOptions(FORM_SCHEMA,"epdFilterField");
    renderFilterChips("epdFilterList",_epdFilters,"removeEpdFilter");

    renderAllowedEditorRespondents(allowedData.respondents||[]);

    const picker=document.getElementById("epdRespondentPicker");
    const allowed=new Set((allowedData.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (respondentsData.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");

    epDetailDlg.showModal();
  }catch(e){adminToast("Failed to load: "+e.message,true);}
}

document.querySelectorAll("input[name='epdRA']").forEach(rb=>{
  rb.addEventListener("change",()=>toggleEditorRespondentSection(rb.value==="selected"));
});

function toggleEditorRespondentSection(show){
  document.getElementById("epdRespondentSection").style.display=show?"block":"none";
}

function renderAllowedEditorRespondents(list){
  const el=document.getElementById("epdRespondentList");
  if(!list.length){el.innerHTML='<div class="muted" style="font-size:11px">No respondents selected yet.</div>';return;}
  el.innerHTML=list.map(r=>`
    <div style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
      <span style="flex:1">${esc(r.name||r.email||r.respondentId)}</span>
      <button class="btn danger" style="font-size:11px;padding:2px 6px" onclick="removeAllowedEditorRespondent('${r.id}')">✕</button>
    </div>`).join("");
}

async function addAllowedEditorRespondent(){
  const respondentId=document.getElementById("epdRespondentPicker").value;
  if(!respondentId)return;
  try{
    await api("/api/editor-permissions/"+_epdPermId+"/respondents",{
      method:"POST",body:JSON.stringify({respondentId})
    });
    const [perm,formRespondents]=await Promise.all([
      api("/api/editor-permissions/"+_epdPermId+"/respondents"),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedEditorRespondents(perm.respondents||[]);
    const picker=document.getElementById("epdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Failed: "+e.message,true);}
}

async function removeAllowedEditorRespondent(id){
  try{
    await api("/api/editor-respondents/"+id,{method:"DELETE"});
    const [perm,formRespondents]=await Promise.all([
      api("/api/editor-permissions/"+_epdPermId+"/respondents"),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedEditorRespondents(perm.respondents||[]);
    const picker=document.getElementById("epdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Failed: "+e.message,true);}
}

async function saveEpDetail(){
  const respondentAccess=document.querySelector("input[name='epdRA']:checked")?.value||"all";
  try{
    const note=(document.getElementById("epdNote").value||"").trim();
    await api("/api/editor-permissions/"+_epdPermId,{
      method:"PUT",body:JSON.stringify({respondentAccess,fieldFilters:_epdFilters,note})
    });
    epDetailDlg.close();
    await refreshUserPermList();
  }catch(e){adminToast("Failed to save: "+e.message,true);}
}

function convertEpToViewer(){
  adminConfirm("Convert this access to Viewer? The existing editor access is removed and replaced by a new viewer access with the same settings.",async()=>{
    try{
      await api("/api/editor-permissions/"+_epdPermId+"/convert-to-viewer",{method:"POST"});
      epDetailDlg.close();
      await refreshUserPermList();
      adminToast("Access switched to viewer");
    }catch(e){adminToast("Failed: "+e.message,true);}
  });
}

let _vpPermCache=[];

async function removeViewerPerm(permId,viewerName){
  adminConfirm(`Revoke access for "${viewerName}" on this form?`,async()=>{
    try{await api("/api/viewer-permissions/"+permId,{method:"DELETE"});await refreshUserPermList();}
    catch(e){adminToast("Failed: "+e.message,true);}
  });
}

/* ======================================================
   USERS — the combined viewer + editor list for this form
   ====================================================== */

async function initUserSection(){
  await refreshUserPermList();
}

async function refreshUserPermList(){
  const el=document.getElementById("userPermList");
  el.innerHTML='<tr><td colspan="6" class="empty">Loading…</td></tr>';
  try{
    const [vRes,eRes]=await Promise.all([
      api("/api/forms/"+FORM_ID+"/viewer-permissions"),
      api("/api/forms/"+FORM_ID+"/editor-permissions"),
    ]);
    _vpPermCache=vRes.permissions||[];
    _epPermCache=eRes.permissions||[];
    renderUserPermTable();
  }catch(e){
    el.innerHTML=`<tr><td colspan="6" class="empty">${esc(e.message)}</td></tr>`;
  }
}

function renderUserPermTable(){
  const el=document.getElementById("userPermList");
  const rows=[
    ..._vpPermCache.map(p=>({...p,role:"viewer"})),
    ..._epPermCache.map(p=>({...p,role:"editor"})),
  ];
  if(!rows.length){
    el.innerHTML='<tr><td colspan="6" class="empty">No users added yet.</td></tr>';
    return;
  }
  el.innerHTML=rows.map(p=>{
    const isViewer=p.role==="viewer";
    const email=isViewer?p.viewerUsername:(p.editorName||"(editor)");
    // The note recorded on the account when it was created — usually who the person is
    // or which office they belong to, which the email alone rarely says.
    const note=(isViewer?p.viewerNote:p.editorNote)||"";
    const respAccess=p.respondentAccess==="all"?"All respondents":`${p.allowedCount} respondents selected`;
    const varAccess=isViewer?(p.visibleFields&&p.visibleFields.length?p.visibleFields.length+" fields":"All fields"):"-";
    const detailFn=isViewer?"openVpDetail":"openEpDetail";
    const removeFn=isViewer?"removeViewerPerm":"removeEditorPerm";
    return`<tr>
      <td>${esc(email)}</td>
      <td class="muted"${note?` title="${esc(note)}"`:""}>${note?esc(note):"—"}</td>
      <td><span class="tag${isViewer?"":" archived"}">${isViewer?"Viewer":"Editor"}</span></td>
      <td class="muted">${respAccess}</td>
      <td class="muted">${varAccess}</td>
      <td class="acts">
        <button class="btn btn-xs" onclick="${detailFn}('${p.id}','${esc(email)}')">Configure</button>
        <button class="btn danger btn-xs" onclick="${removeFn}('${p.id}','${esc(email)}')">Delete</button>
      </td>
    </tr>`;
  }).join("");
}

let _uaFilters={};
let _uaSelectedRespondents=[];
let _uaAllRespondents=[];

function uaRoleChanged(){
  const isViewer=document.querySelector("input[name='uaRole']:checked")?.value!=="editor";
  document.getElementById("uaFieldListSection").style.display=isViewer?"block":"none";
}

function toggleUaRespondentSection(show){
  document.getElementById("uaRespondentSection").style.display=show?"block":"none";
}

function renderUaRespondents(){
  const el=document.getElementById("uaRespondentList");
  if(!_uaSelectedRespondents.length){el.innerHTML='<div class="muted" style="font-size:11px">No respondents selected yet.</div>';return;}
  el.innerHTML=_uaSelectedRespondents.map(r=>`
    <div style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
      <span style="flex:1">${esc(r.name||r.email||r.id)}</span>
      <button class="btn danger" type="button" style="font-size:11px;padding:2px 6px" onclick="removeUaRespondent('${r.id}')">✕</button>
    </div>`).join("");
  const picker=document.getElementById("uaRespondentPicker");
  const chosen=new Set(_uaSelectedRespondents.map(r=>r.id));
  picker.innerHTML=`<option value="">— select respondent —</option>`+
    _uaAllRespondents.filter(r=>!chosen.has(r.id)).map(r=>
      `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
}

function addUaRespondent(){
  const id=document.getElementById("uaRespondentPicker").value;
  if(!id)return;
  const r=_uaAllRespondents.find(x=>x.id===id);
  if(!r)return;
  _uaSelectedRespondents.push(r);
  renderUaRespondents();
}

function removeUaRespondent(id){
  _uaSelectedRespondents=_uaSelectedRespondents.filter(r=>r.id!==id);
  renderUaRespondents();
}

function uaCheckAll(on){
  document.querySelectorAll("#uaFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
}

function addUaFilter(){
  const field=document.getElementById("uaFilterField").value;
  const value=(document.getElementById("uaFilterValue").value||"").trim();
  if(!field||!value){adminToast("Select a field and enter a value",true);return;}
  _uaFilters[field]=value;
  document.getElementById("uaFilterValue").value="";
  renderFilterChips("uaFilterList",_uaFilters,"removeUaFilter");
}
function removeUaFilter(field){
  delete _uaFilters[field];
  renderFilterChips("uaFilterList",_uaFilters,"removeUaFilter");
}

async function openUserAddDlg(){
  document.querySelector("input[name='uaRole'][value='viewer']").checked=true;
  document.getElementById("uaEmail").value="";
  document.getElementById("uaNote").value="";
  uaRoleChanged();

  document.querySelector("input[name='uaRA'][value='all']").checked=true;
  toggleUaRespondentSection(false);
  _uaSelectedRespondents=[];
  _uaAllRespondents=[];
  renderUaRespondents();
  document.getElementById("uaRespondentPicker").innerHTML='<option value="">— select respondent —</option>';

  buildFieldCheckboxes("uaFieldList",FORM_SCHEMA,[]);

  _uaFilters={};
  buildFieldOptions(FORM_SCHEMA,"uaFilterField");
  document.getElementById("uaFilterField").value="";
  document.getElementById("uaFilterValue").value="";
  renderFilterChips("uaFilterList",_uaFilters,"removeUaFilter");

  userAddDlg.showModal();

  try{
    const{respondents}=await api("/api/forms/"+FORM_ID+"/respondents");
    _uaAllRespondents=respondents||[];
    renderUaRespondents();
  }catch(_){}
}

async function submitUserAdd(){
  const role=document.querySelector("input[name='uaRole']:checked")?.value||"viewer";
  const email=(document.getElementById("uaEmail").value||"").trim().toLowerCase();
  const note=(document.getElementById("uaNote").value||"").trim();
  const respondentAccess=document.querySelector("input[name='uaRA']:checked")?.value||"all";
  const fieldFilters={..._uaFilters};
  let visibleFields=[];
  if(role==="viewer"){
    const checked=[...document.querySelectorAll("#uaFieldList input:checked")].map(cb=>cb.value);
    const total=document.querySelectorAll("#uaFieldList input").length;
    visibleFields=checked.length===total?[]:checked;
  }
  if(!email){adminToast("Email is required",true);return;}
  if(!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)){adminToast("Invalid email format",true);return;}
  const btn=document.getElementById("uaSaveBtn");
  btn.disabled=true;btn.textContent="Saving…";
  try{
    const path=role==="viewer"
      ?"/api/forms/"+FORM_ID+"/viewer-permissions/bulk"
      :"/api/forms/"+FORM_ID+"/editor-permissions/bulk";
    const item=role==="viewer"
      ?{email,note,respondentAccess,visibleFields,fieldFilters}
      :{email,note,respondentAccess,fieldFilters};
    const{results}=await api(path,{method:"POST",body:JSON.stringify({items:[item]})});
    const res=results&&results[0];
    if(res&&res.status==="error"){adminToast("Failed: "+res.error,true);return;}
    if(respondentAccess==="selected"&&_uaSelectedRespondents.length&&res.permissionId){
      const respPath=role==="viewer"
        ?"/api/viewer-permissions/"+res.permissionId+"/respondents"
        :"/api/editor-permissions/"+res.permissionId+"/respondents";
      await Promise.all(_uaSelectedRespondents.map(r=>
        api(respPath,{method:"POST",body:JSON.stringify({respondentId:r.id})}).catch(()=>{})
      ));
    }
    userAddDlg.close();
    await refreshUserPermList();
    adminToast("User ditambahkan");
  }catch(e){adminToast("Failed: "+e.message,true);}
  finally{btn.disabled=false;btn.textContent="Add";}
}

let _vpdPermId=null;
async function openVpDetail(permId,viewerName){
  _vpdPermId=permId;
  document.getElementById("vpdViewerName").textContent=viewerName;
  // Taken from the row already on screen: the detail endpoint reads only the
  // permission, while the note belongs to the account and arrives with the list.
  document.getElementById("vpdNote").value=
    (_vpPermCache.find(p=>p.id===permId)||{}).viewerNote||"";

  try{
    const [curPerm,allowedData,respondentsData]=await Promise.all([
      api("/api/viewer-permissions/"+permId),
      api("/api/viewer-permissions/"+permId+"/respondents").catch(()=>({respondents:[]})),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);

    document.querySelector(`input[name='vpdRA'][value='${curPerm.respondentAccess}']`).checked=true;
    toggleRespondentSection(curPerm.respondentAccess==="selected");

    buildVpdFieldList(FORM_SCHEMA,curPerm.visibleFields||[]);

    _vpdFilters=curPerm.fieldFilters||{};
    buildFieldOptions(FORM_SCHEMA,"vpdFilterField");
    renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");

    renderAllowedRespondents(allowedData.respondents||[]);

    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((allowedData.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (respondentsData.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");

    vpDetailDlg.showModal();
  }catch(e){adminToast("Failed to load: "+e.message,true);}
}

document.querySelectorAll("input[name='vpdRA']").forEach(rb=>{
  rb.addEventListener("change",()=>toggleRespondentSection(rb.value==="selected"));
});

function toggleRespondentSection(show){
  document.getElementById("vpdRespondentSection").style.display=show?"block":"none";
}

function renderAllowedRespondents(list){
  const el=document.getElementById("vpdRespondentList");
  if(!list.length){el.innerHTML='<div class="muted" style="font-size:11px">No respondents selected yet.</div>';return;}
  el.innerHTML=list.map(r=>`
    <div style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
      <span style="flex:1">${esc(r.name||r.email||r.respondentId)}</span>
      <button class="btn danger" style="font-size:11px;padding:2px 6px" onclick="removeAllowedRespondent('${r.id}')">✕</button>
    </div>`).join("");
}

async function addAllowedRespondent(){
  const respondentId=document.getElementById("vpdRespondentPicker").value;
  if(!respondentId)return;
  try{
    await api("/api/viewer-permissions/"+_vpdPermId+"/respondents",{
      method:"POST",body:JSON.stringify({respondentId})
    });
    const [perm,formRespondents]=await Promise.all([
      api("/api/viewer-permissions/"+_vpdPermId+"/respondents"),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedRespondents(perm.respondents||[]);
    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Failed: "+e.message,true);}
}

async function removeAllowedRespondent(id){
  try{
    await api("/api/viewer-respondents/"+id,{method:"DELETE"});
    const [perm,formRespondents]=await Promise.all([
      api("/api/viewer-permissions/"+_vpdPermId+"/respondents"),
      api("/api/forms/"+FORM_ID+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedRespondents(perm.respondents||[]);
    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— select respondent —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Failed: "+e.message,true);}
}

function buildFieldCheckboxes(containerId,schema,checked){
  const el=document.getElementById(containerId);
  if(!el)return;
  const fields=[];
  function walk(comps){
    for(const c of comps||[]){
      if(c.kind==="field"&&c.name&&c.type!=="note"&&c.type!=="hidden"&&c.type!=="markdown")
        fields.push({name:c.name,label:typeof c.label==="string"?c.label:(c.label?.id||c.name)});
      else if(c.components)walk(c.components);
    }
  }
  for(const p of schema?.pages||[])walk(p.components||[]);
  if(!fields.length){el.innerHTML='<div style="font-size:12px;color:var(--muted)">There are no fields in this form.</div>';return;}
  el.innerHTML=fields.map(f=>`
    <label style="display:flex;align-items:center;gap:8px;padding:4px 0;font-size:12px;cursor:pointer">
      <input type="checkbox" value="${esc(f.name)}" ${!checked.length||checked.includes(f.name)?"checked":""}>
      <span>${esc(f.label)}</span>
    </label>`).join("");
}

function buildVpdFieldList(schema,checked){buildFieldCheckboxes("vpdFieldList",schema,checked);}

function vpdCheckAll(on){
  document.querySelectorAll("#vpdFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
}

async function savePermDetail(){
  const respondentAccess=document.querySelector("input[name='vpdRA']:checked")?.value||"all";
  const checked=[...document.querySelectorAll("#vpdFieldList input:checked")].map(cb=>cb.value);
  const total=document.querySelectorAll("#vpdFieldList input").length;
  const visibleFields=checked.length===total?[]:checked;
  try{
    const note=(document.getElementById("vpdNote").value||"").trim();
    await api("/api/viewer-permissions/"+_vpdPermId,{
      method:"PUT",body:JSON.stringify({respondentAccess,visibleFields,fieldFilters:_vpdFilters,note})
    });
    vpDetailDlg.close();
    await refreshUserPermList();
  }catch(e){adminToast("Failed to save: "+e.message,true);}
}

function convertVpToEditor(){
  adminConfirm("Convert this access to Editor? The existing viewer access is removed and replaced by a new editor access with the same settings.",async()=>{
    try{
      await api("/api/viewer-permissions/"+_vpdPermId+"/convert-to-editor",{method:"POST"});
      vpDetailDlg.close();
      await refreshUserPermList();
      adminToast("Access switched to editor");
    }catch(e){adminToast("Failed: "+e.message,true);}
  });
}

/* ======================================================
   API — keys for external systems to pull this form's responses
   ====================================================== */

let _akEditingId=null;            // null = currently creating a new key
let _akFilters={};
let _akSelectedRespondents=[];
let _akAllRespondents=[];
let _akRevealedKey="";

function initApiSection(){
  document.getElementById("apiDocPre").textContent=apiUsageSnippet("EFORM_API_KEY");
  refreshApiKeys();
}

// apiUsageSnippet builds a curl example using the real key (right after creation) or
// placeholder (in the documentation at the bottom).
function apiUsageSnippet(key){
  const base=location.origin;
  return `curl -H "Authorization: Bearer ${key}" \\
  "${base}/api/v1/forms/${FORM_ID}/responses?limit=50"

# CSV
curl -H "Authorization: Bearer ${key}" \\
  "${base}/api/v1/forms/${FORM_ID}/responses.csv" -o responses.csv

# check the key configuration without pulling data
curl -H "Authorization: Bearer ${key}" "${base}/api/v1/me"`;
}

function akStatus(k){
  if(!k.isActive) return{cls:"archived",text:"Inactive"};
  if(k.expiresAt&&new Date(k.expiresAt)<new Date()) return{cls:"archived",text:"Expires"};
  return{cls:"published",text:"Active"};
}

async function refreshApiKeys(){
  const el=document.getElementById("apiKeyList");
  try{
    const{apiKeys}=await api("/api/forms/"+FORM_ID+"/api-keys");
    document.getElementById("apiDoc").hidden=!(apiKeys&&apiKeys.length);
    if(!apiKeys||!apiKeys.length){
      el.innerHTML='<div class="share-empty muted">No API keys yet. Click "+ Create API Key" to create the first one.</div>';
      return;
    }
    el.innerHTML=apiKeys.map(k=>{
      const st=akStatus(k);
      const badges=[];
      badges.push(`<span class="tag">${k.respondentAccess==="all"?"All respondents":`${k.allowedCount||0} respondents`}</span>`);
      badges.push(`<span class="tag">${k.visibleFields&&k.visibleFields.length?`${k.visibleFields.length} fields`:"All fields"}</span>`);
      if(!k.includeRespondent) badges.push('<span class="tag">No identity</span>');
      if(k.allowedIps&&k.allowedIps.length) badges.push(`<span class="tag">${k.allowedIps.length} IP</span>`);
      if(k.fieldFilters&&Object.keys(k.fieldFilters).length) badges.push(`<span class="tag">${Object.keys(k.fieldFilters).length} filter</span>`);
      const used=k.lastUsedAt
        ? `Last used ${new Date(k.lastUsedAt).toLocaleString("id-ID")}${k.lastUsedIp?" from "+esc(k.lastUsedIp):""}`
        : "Never used";
      return`<div class="share-card">
        <div class="share-card-top">
          <div class="share-card-title">
            <b>${esc(k.label||"(no label)")}</b>
            <span class="tag ${st.cls}">${st.text}</span>
          </div>
        </div>
        <div class="share-badges">${badges.join("")}</div>
        <div class="share-url-row"><code class="share-url">eform_${esc(k.keyPrefix)}…</code></div>
        <div class="share-meta muted">${used} · ${k.requestCount||0}× requests${k.expiresAt?` · valid until ${new Date(k.expiresAt).toLocaleString()}`:""}</div>
        <div class="acts" style="margin-top:10px">
          <button class="btn" type="button" onclick="openApiKeyDlg('${k.id}')">Configure</button>
          <button class="btn" type="button" onclick="openApiLogDlg('${k.id}','${esc(k.label||k.keyPrefix)}')">Access Log</button>
          <button class="btn" type="button" onclick="rotateApiKey('${k.id}','${esc(k.label||k.keyPrefix)}')">Rotate</button>
          <button class="btn danger" type="button" onclick="deleteApiKey('${k.id}','${esc(k.label||k.keyPrefix)}')">Delete</button>
        </div>
      </div>`;
    }).join("");
  }catch(e){ el.innerHTML=`<div class="share-empty muted">${esc(e.message)}</div>`; }
}

function toggleAkRespondentSection(show){
  document.getElementById("akRespondentSection").style.display=show?"block":"none";
}

function akCheckAll(on){
  document.querySelectorAll("#akFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
}

function renderAkRespondents(){
  const el=document.getElementById("akRespondentList");
  if(!_akSelectedRespondents.length){el.innerHTML='<div class="muted" style="font-size:11px">No respondents selected yet.</div>';}
  else{
    el.innerHTML=_akSelectedRespondents.map(r=>`
      <div style="display:flex;align-items:center;gap:6px;padding:3px 0;font-size:12px">
        <span style="flex:1">${esc(r.name||r.email||r.id)}</span>
        <button class="btn danger" type="button" style="font-size:11px;padding:2px 6px" onclick="removeAkRespondent('${r.id}')">✕</button>
      </div>`).join("");
  }
  const picker=document.getElementById("akRespondentPicker");
  const chosen=new Set(_akSelectedRespondents.map(r=>r.id));
  picker.innerHTML=`<option value="">— select respondent —</option>`+
    _akAllRespondents.filter(r=>!chosen.has(r.id)).map(r=>
      `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
}

function addAkRespondent(){
  const id=document.getElementById("akRespondentPicker").value;
  if(!id)return;
  const r=_akAllRespondents.find(x=>x.id===id);
  if(!r)return;
  _akSelectedRespondents.push(r);
  renderAkRespondents();
}

function removeAkRespondent(id){
  _akSelectedRespondents=_akSelectedRespondents.filter(r=>r.id!==id);
  renderAkRespondents();
}

function addAkFilter(){
  const field=document.getElementById("akFilterField").value;
  const value=(document.getElementById("akFilterValue").value||"").trim();
  if(!field||!value){adminToast("Select a field and enter a value",true);return;}
  _akFilters[field]=value;
  document.getElementById("akFilterValue").value="";
  renderFilterChips("akFilterList",_akFilters,"removeAkFilter");
}
function removeAkFilter(field){
  delete _akFilters[field];
  renderFilterChips("akFilterList",_akFilters,"removeAkFilter");
}

// openApiKeyDlg serves both creation (empty keyId) and editing an existing key.
async function openApiKeyDlg(keyId){
  _akEditingId=keyId||null;
  const isEdit=!!_akEditingId;
  document.getElementById("akDlgTitle").textContent=isEdit?"Configure API Key":"Create API Key";
  document.getElementById("akSaveBtn").textContent=isEdit?"Save":"Create API Key";
  document.getElementById("akActiveRow").style.display=isEdit?"flex":"none";

  let k={label:"",respondentAccess:"all",visibleFields:[],fieldFilters:{},
         includeRespondent:false,allowedIps:[],rateLimitPerMin:60,isActive:true,expiresAt:null};
  let allowedRespondents=[];
  if(isEdit){
    try{
      const[list,allowed]=await Promise.all([
        api("/api/forms/"+FORM_ID+"/api-keys"),
        api("/api/api-keys/"+_akEditingId+"/respondents").catch(()=>({respondents:[]}))
      ]);
      const found=(list.apiKeys||[]).find(x=>x.id===_akEditingId);
      if(found) k=found;
      allowedRespondents=(allowed.respondents||[]).map(r=>({id:r.respondentId,name:r.name,email:r.email}));
    }catch(e){adminToast("Failed to load: "+e.message,true);return;}
  }

  document.getElementById("akLabel").value=k.label||"";
  document.querySelector(`input[name='akRA'][value='${k.respondentAccess||"all"}']`).checked=true;
  toggleAkRespondentSection(k.respondentAccess==="selected");
  document.getElementById("akIncludeRespondent").checked=!!k.includeRespondent;
  document.getElementById("akAllowedIps").value=(k.allowedIps||[]).join(", ");
  document.getElementById("akRateLimit").value=k.rateLimitPerMin||60;
  document.getElementById("akIsActive").checked=k.isActive!==false;
  document.getElementById("akExpiresAt").value=toLocalDT(k.expiresAt);

  buildFieldCheckboxes("akFieldList",FORM_SCHEMA,k.visibleFields||[]);

  _akFilters={...(k.fieldFilters||{})};
  buildFieldOptions(FORM_SCHEMA,"akFilterField");
  document.getElementById("akFilterField").value="";
  document.getElementById("akFilterValue").value="";
  renderFilterChips("akFilterList",_akFilters,"removeAkFilter");

  _akSelectedRespondents=allowedRespondents;
  _akAllRespondents=[];
  renderAkRespondents();

  apiKeyDlg.showModal();

  try{
    const{respondents}=await api("/api/forms/"+FORM_ID+"/respondents");
    _akAllRespondents=respondents||[];
    renderAkRespondents();
  }catch(e){}
}

// collectApiKeyForm reads the whole dialog into an API payload.
function collectApiKeyForm(){
  const checked=[...document.querySelectorAll("#akFieldList input:checked")].map(cb=>cb.value);
  const total=document.querySelectorAll("#akFieldList input").length;
  const expiresRaw=document.getElementById("akExpiresAt").value;
  return{
    label:document.getElementById("akLabel").value.trim(),
    respondentAccess:document.querySelector("input[name='akRA']:checked")?.value||"all",
    // Everything ticked = no column restriction, exactly like a viewer permission.
    visibleFields:checked.length===total?[]:checked,
    fieldFilters:_akFilters,
    includeRespondent:document.getElementById("akIncludeRespondent").checked,
    allowedIps:document.getElementById("akAllowedIps").value.split(",").map(s=>s.trim()).filter(Boolean),
    rateLimitPerMin:parseInt(document.getElementById("akRateLimit").value,10)||60,
    isActive:document.getElementById("akIsActive").checked,
    expiresAt:expiresRaw?new Date(expiresRaw).toISOString():""
  };
}

async function submitApiKey(){
  const btn=document.getElementById("akSaveBtn");
  const body=collectApiKeyForm();
  btn.disabled=true;
  try{
    if(_akEditingId){
      await api("/api/api-keys/"+_akEditingId,{method:"PUT",body:JSON.stringify(body)});
      await syncAkRespondents(_akEditingId);
      apiKeyDlg.close();
      adminToast("API key updated");
    }else{
      const res=await api("/api/forms/"+FORM_ID+"/api-keys",{method:"POST",body:JSON.stringify(body)});
      await syncAkRespondents(res.apiKey.id);
      apiKeyDlg.close();
      showApiKeyReveal(res.key);
    }
    await refreshApiKeys();
  }catch(e){adminToast("Failed: "+e.message,true);}
  finally{btn.disabled=false;}
}

// syncAkRespondents reconciles the respondent list on the server with the dialog's selection.
async function syncAkRespondents(keyId){
  if(document.querySelector("input[name='akRA']:checked")?.value!=="selected") return;
  let existing=[];
  try{
    const d=await api("/api/api-keys/"+keyId+"/respondents");
    existing=d.respondents||[];
  }catch(e){}
  const wanted=new Set(_akSelectedRespondents.map(r=>r.id));
  const have=new Set(existing.map(r=>r.respondentId));
  for(const r of _akSelectedRespondents){
    if(!have.has(r.id)){
      try{await api("/api/api-keys/"+keyId+"/respondents",{method:"POST",body:JSON.stringify({respondentId:r.id})});}catch(e){}
    }
  }
  for(const r of existing){
    if(!wanted.has(r.respondentId)){
      try{await api("/api/api-key-respondents/"+r.id,{method:"DELETE"});}catch(e){}
    }
  }
}

function showApiKeyReveal(key){
  _akRevealedKey=key;
  document.getElementById("akRevealKey").textContent=key;
  document.getElementById("akRevealCurl").textContent=apiUsageSnippet(key);
  apiKeyRevealDlg.showModal();
}

function copyApiKey(){
  navigator.clipboard.writeText(_akRevealedKey)
    .then(()=>adminToast("API key copied"))
    .catch(()=>adminToast("Copy failed",true));
}

function rotateApiKey(id,label){
  adminConfirm(`Rotate API key "${label}"? The old key stops working immediately and any system using it must be updated.`,async()=>{
    try{
      const res=await api("/api/api-keys/"+id+"/rotate",{method:"POST"});
      await refreshApiKeys();
      showApiKeyReveal(res.key);
    }catch(e){adminToast("Failed: "+e.message,true);}
  });
}

function deleteApiKey(id,label){
  adminConfirm(`Delete API key "${label}"? Any system using it loses access immediately.`,async()=>{
    try{
      await api("/api/api-keys/"+id,{method:"DELETE"});
      await refreshApiKeys();
      adminToast("API key deleted");
    }catch(e){adminToast("Failed: "+e.message,true);}
  });
}

async function openApiLogDlg(id,label){
  document.getElementById("alKeyLabel").textContent=label;
  const rows=document.getElementById("apiLogRows");
  rows.innerHTML='<tr><td colspan="5" class="empty">Loading…</td></tr>';
  apiLogDlg.showModal();
  try{
    const{logs}=await api("/api/api-keys/"+id+"/logs?limit=100");
    if(!logs||!logs.length){
      rows.innerHTML='<tr><td colspan="5" class="empty">No API calls yet.</td></tr>';
      return;
    }
    rows.innerHTML=logs.map(l=>{
      const ok=l.status>=200&&l.status<300;
      return`<tr>
        <td>${new Date(l.createdAt).toLocaleString("id-ID")}</td>
        <td class="muted">${esc(l.ip||"-")}</td>
        <td class="muted" style="word-break:break-all">${esc(l.path||"-")}</td>
        <td><span class="tag ${ok?"published":"archived"}">${l.status}${l.error?" · "+esc(l.error):""}</span></td>
        <td class="muted">${l.rowCount||0}</td>
      </tr>`;
    }).join("");
  }catch(e){
    rows.innerHTML=`<tr><td colspan="5" class="empty">${esc(e.message)}</td></tr>`;
  }
}

/* Halaman pengelolaan satu kuesioner (Ringkasan/Bagikan/Akses Viewer/Akses Editor).
   Mandiri seperti halaman lain di app ini (builder-bridge.js, admin.js) — helper
   dasar (api/esc/adminToast/adminConfirm) sengaja diduplikasi, bukan diimpor. */
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
  if(r.status===401){localStorage.removeItem("eform_token");location.replace("/login");throw new Error("sesi habis");}
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
    adminToast("Gagal memuat kuesioner: "+e.message,true);
  }
}

async function renderOverview(f){
  const statusEl=document.getElementById("ovStatus");
  statusEl.textContent=f.status;
  statusEl.className="tag "+f.status;
  document.getElementById("ovUpdated").textContent="Diperbarui "+new Date(f.updatedAt).toLocaleString("id-ID");
  const pubBtn=document.getElementById("ovPubBtn");
  pubBtn.textContent=f.status==="published"?"Tarik":"Publikasikan";
  const respEl=document.getElementById("ovResponses");
  const delBtn=document.getElementById("ovDeleteBtn");
  respEl.textContent="Memuat jumlah jawaban…";
  try{
    const d=await api("/api/forms/"+FORM_ID+"/responses?limit=1");
    respEl.textContent=d.total+" jawaban";
    delBtn.disabled=d.total>0;
    delBtn.title=d.total>0?"Tidak dapat dihapus karena sudah ada jawaban":"";
  }catch(e){respEl.textContent="";}
}

async function togglePub(){
  const next=FORM_STATUS==="published"?"draft":"published";
  try{await api("/api/forms/"+FORM_ID+"/publish",{method:"POST",body:JSON.stringify({status:next})});await loadForm();}
  catch(e){adminToast(e.message,true);}
}

function delForm(){
  adminConfirm("Hapus kuesioner \""+FORM_TITLE+"\"? Kuesioner hanya dapat dihapus jika belum ada jawaban.",async()=>{
    try{await api("/api/forms/"+FORM_ID,{method:"DELETE"});location.href="/admin";}catch(e){adminToast(e.message,true);}
  });
}

/* ======================================================
   SIDEBAR — pindah antar section (lazy-load data di kunjungan pertama)
   ====================================================== */

let _shareInited=false,_viewerInited=false,_editorInited=false;
function switchSection(sec){
  ["overview","share","viewer","editor"].forEach(s=>{
    const el=document.getElementById("sec-"+s);
    const btn=document.getElementById("nav-"+s);
    if(el) el.hidden=s!==sec;
    if(btn) btn.classList.toggle("active",s===sec);
  });
  const shown=document.getElementById("sec-"+sec);
  if(shown){shown.classList.add("fade-in");setTimeout(()=>shown.classList.remove("fade-in"),200);}
  if(sec==="overview") loadForm();
  else if(sec==="share"&&!_shareInited){_shareInited=true;initShareSection();}
  else if(sec==="viewer"&&!_viewerInited){_viewerInited=true;initViewerSection();}
  else if(sec==="editor"&&!_editorInited){_editorInited=true;initEditorSection();}
}

/* ======================================================
   BAGIKAN (dulu shareDlg)
   ====================================================== */

function initShareSection(){
  refreshShares();
}

function openShareCreateDlg(){
  $("#shareNote").innerHTML = FORM_STATUS==="published"
    ? "Kuesioner sudah <b>published</b> — tautan bisa langsung diakses publik."
    : "⚠️ Kuesioner masih <b>draft</b>. Tautan dibuat, tapi publik baru bisa membuka setelah dipublikasikan.";
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
    :"<div class='muted' style='font-size:12px;padding:4px 0'>Belum ada email ditambahkan.</div>";
}
function removePending(i){pendingEmails.splice(i,1);renderPendingEmails();}
$("#btnAddNewEmail").addEventListener("click",()=>{
  const email=$("#newEmailInput").value.trim().toLowerCase();
  const note=$("#newEmailNote").value.trim();
  if(!email){$("#newEmailInput").focus();return;}
  if(pendingEmails.some(e=>e.email===email)){adminToast("Email sudah ada di daftar",true);return;}
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
  if(btn){btn.disabled=true;btn.textContent="Menyimpan…";}
  try{
    await api("/api/shares/"+id,{method:"PATCH",body:JSON.stringify({
      label,allowResponses,multiResponse,accessMode,
      updatePassword,password,
      updateExpiry:true,expiresAt
    })});
    editingShareId=null;refreshShares();
  }catch(e){adminToast(e.message,true);if(btn){btn.disabled=false;btn.textContent="Simpan";}}
}

const ICON_LOCK='<svg viewBox="0 0 20 20" width="11" height="11" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect x="4.5" y="9" width="11" height="8" rx="1.5"/><path d="M7 9V6a3 3 0 0 1 6 0v3"/></svg>';
const ICON_COPY='<svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="7" y="7" width="10" height="10" rx="1.5"/><path d="M4.5 13H3.5A1.5 1.5 0 0 1 2 11.5v-8A1.5 1.5 0 0 1 3.5 2h8A1.5 1.5 0 0 1 13 3.5v1"/></svg>';
const ICON_EYE_SM='<svg viewBox="0 0 20 20" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M1.5 10 Q10 2 18.5 10 Q10 18 1.5 10 Z"/><circle cx="10" cy="10" r="2.3"/></svg>';

function copyShareUrl(url){
  navigator.clipboard.writeText(url).then(()=>adminToast("Tautan disalin")).catch(()=>adminToast("Gagal menyalin",true));
}

async function refreshShares(){
  try{
    const {shares}=await api("/api/forms/"+FORM_ID+"/shares");
    if(!shares||!shares.length){
      $("#shareList").innerHTML='<div class="share-empty muted">Belum ada tautan share. Klik "+ Buat Tautan Share" untuk membuat yang pertama.</div>';
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
      if(s.multiResponse)badges.push('<span class="tag">Multi-respons</span>');
      if(s.accessMode==="restricted")badges.push('<span class="tag">Terbatas</span>');

      const editSection=isEditing?`<div class="share-edit">
        <div class="edit-row"><span class="edit-lbl">Label</span>
          <input id="elabel_${s.id}" value="${esc(s.label||"")}" style="flex:1">
        </div>
        <div class="edit-row" style="gap:16px;flex-wrap:wrap">
          <label class="muted"><input type="checkbox" id="eallow_${s.id}" ${s.allowResponses?"checked":""}> Terima jawaban</label>
          <label class="muted"><input type="checkbox" id="emulti_${s.id}" ${s.multiResponse?"checked":""}> Multi-respons</label>
        </div>
        <div class="edit-row" style="gap:16px;flex-wrap:wrap">
          <span class="edit-lbl">Akses</span>
          <label class="muted"><input type="radio" name="eacc_${s.id}" value="public" ${s.accessMode!=="restricted"?"checked":""}> Publik</label>
          <label class="muted"><input type="radio" name="eacc_${s.id}" value="restricted" ${s.accessMode==="restricted"?"checked":""}> Terbatas</label>
        </div>
        <div class="edit-row"><span class="edit-lbl">Password baru</span>
          <input id="epw_${s.id}" type="text" placeholder="${s.hasPassword?"Password sudah diatur — isi untuk ubah":"Opsional"}" style="flex:1">
        </div>
        ${s.hasPassword?`<div class="edit-row"><span class="edit-lbl"></span>
          <label class="muted"><input type="checkbox" id="eclearpw_${s.id}"> Hapus password yang ada</label>
        </div>`:""}
        <div class="edit-row"><span class="edit-lbl">Kedaluwarsa</span>
          <input id="eexp_${s.id}" type="datetime-local" value="${toLocalDT(s.expiresAt)}" style="flex:1">
          <span class="muted" style="font-size:11px">Kosongkan = tidak ada batas</span>
        </div>
        <div class="acts" style="margin-top:10px">
          <button class="btn primary btn-sm" id="esave_${s.id}" onclick="saveShareEdit('${s.id}',${s.hasPassword})">Simpan</button>
          <button class="btn btn-sm" onclick="cancelEdit()">Batal</button>
        </div>
      </div>`:"";

      let emailSection="";
      if(s.accessMode==="restricted"&&!isEditing){
        const emails=emailMap[s.id]||[];
        const rows=emails.length
          ?emails.map(e=>`<tr><td>${esc(e.email)}</td><td class="muted">${esc(e.note)}</td><td><button class="btn danger btn-xs" onclick="removeEmail('${e.id}')">✕</button></td></tr>`).join("")
          :`<tr><td colspan="3" class="muted" style="padding:6px 0">Belum ada akun terdaftar.</td></tr>`;
        emailSection=`<div class="email-sect">
          <div class="email-sect-h">Akun yang diizinkan (${emails.length})</div>
          <table class="email-tbl"><tbody>${rows}</tbody></table>
          <div class="row" style="gap:6px;margin-top:8px">
            <input id="addIn_${s.id}" type="email" placeholder="email@contoh.com" style="flex:2">
            <input id="addNote_${s.id}" placeholder="Catatan" style="flex:2">
            <button class="btn btn-xs" onclick="addEmailToShare('${s.id}')">+ Tambah</button>
          </div>
        </div>`;
      }

      return `<div class="share-card">
        <div class="share-card-top">
          <div class="share-card-title">
            <b>${esc(s.label||"(tanpa label)")}</b>
            <span class="tag ${s.isActive?"published":"archived"}">${s.isActive?"Aktif":"Nonaktif"}</span>
          </div>
          ${s.isActive&&!isEditing?`<button class="btn btn-xs" onclick="startEdit('${s.id}')">Edit</button>`:""}
        </div>
        ${badges.length?`<div class="share-badges">${badges.join("")}</div>`:""}
        <div class="share-url-row">
          <code class="share-url">${esc(s.shareUrl)}</code>
          <button class="share-copy-btn" type="button" title="Salin tautan" onclick="copyShareUrl('${esc(s.shareUrl)}')">${ICON_COPY}</button>
        </div>
        <div class="share-meta muted">${ICON_EYE_SM} ${s.viewCount}× dibuka</div>
        ${editSection}${emailSection}
        <div class="acts" style="margin-top:10px">
          <a class="btn" href="${esc(s.shareUrl)}" target="_blank">Buka</a>
          ${!isEditing?(s.isActive
            ?`<button class="btn danger" onclick="revoke('${s.id}')">Cabut</button>`
            :`<button class="btn" onclick="reactivateShare('${s.id}')">Aktifkan Kembali</button><button class="btn danger" onclick="deleteShare('${s.id}')">Hapus</button>`
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
  try{await api("/api/shares/"+id+"/reactivate",{method:"POST"});refreshShares();adminToast("Tautan diaktifkan kembali");}
  catch(e){adminToast(e.message,true);}
}
async function deleteShare(id){
  adminConfirm("Hapus permanen tautan ini beserta semua konfigurasinya?",async()=>{
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
    adminToast("Tautan share dibuat");
  }catch(e){adminToast(e.message,true);}
});

/* ======================================================
   AKSES EDITOR (dulu editorPermDlg)
   ====================================================== */

async function initEditorSection(){
  await refreshEditorsAndPerms();
}

async function refreshEditorsAndPerms(){
  await Promise.all([refreshEditorList(), refreshEditorPermList()]);
}

async function refreshEditorList(){
  const el=document.getElementById("editorList");
  const sel=document.getElementById("epEditorSel");
  if(!el||!sel) return;
  el.textContent="Memuat…";
  try{
    const {editors}=await api("/api/editors");
    const cur=sel.value;
    sel.innerHTML='<option value="">— pilih editor —</option>'+
      (editors||[]).map(u=>`<option value="${esc(u.id)}">${esc(u.username)}</option>`).join("");
    sel.value=cur;

    if(!editors||!editors.length){
      el.innerHTML='<div class="muted" style="font-size:13px">Belum ada editor.</div>';
      return;
    }

    el.innerHTML=`<table style="width:100%;border-collapse:collapse;font-size:13px">
      <thead><tr style="background:var(--surface)">
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Username</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Email</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Catatan</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Status</th>
        <th style="padding:6px 8px;border-bottom:1px solid var(--line)"></th>
      </tr></thead>
      <tbody>${editors.map(e=>`<tr>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2)">${esc(e.username)}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);color:var(--muted)">${esc(e.email||"—")}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);color:var(--muted)">${esc(e.note||"—")}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2)"><span class="tag ${e.isActive?"published":"archived"}">${e.isActive?"Aktif":"Nonaktif"}</span></td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);text-align:right">
          <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteEditor('${e.id}','${esc(e.username)}')">Hapus</button>
        </td>
      </tr>`).join("")}</tbody>
    </table>`;
  }catch(e){
    el.textContent="Gagal: "+e.message;
  }
}

async function createEditor(){
  const email=(document.getElementById("eEmail")?.value||"").trim();
  const note=(document.getElementById("eNote")?.value||"").trim();
  if(!email){adminToast("Email Google wajib diisi",true);return;}
  try{
    await api("/api/editors",{method:"POST",body:JSON.stringify({email,note})});
    if(document.getElementById("eEmail"))document.getElementById("eEmail").value="";
    if(document.getElementById("eNote"))document.getElementById("eNote").value="";
    await refreshEditorsAndPerms();
  }catch(e){
    adminToast("Gagal: "+e.message,true);
  }
}

async function deleteEditor(id,name){
  adminConfirm(`Hapus editor "${name}"? Semua akses form editor ini akan ikut dihapus.`,async()=>{
    try{
      await api("/api/editors/"+id,{method:"DELETE"});
      await refreshEditorsAndPerms();
    }catch(e){
      adminToast("Gagal: "+e.message,true);
    }
  });
}

let _epPermCache=[];
let _epPermSelected=new Set();
function bulkEpUpdateSelToolbar(){
  const n=_epPermSelected.size;
  const mb=document.getElementById("epManageSelBtn"),db=document.getElementById("epDeleteSelBtn");
  if(mb){mb.disabled=n===0;mb.textContent=n?`Kelola Terpilih (${n})`:"Kelola Terpilih";}
  if(db){db.disabled=n===0;db.textContent=n?`Hapus Terpilih (${n})`:"Hapus Terpilih";}
}
function bulkEpToggleSel(id,checked){
  if(checked)_epPermSelected.add(id);else _epPermSelected.delete(id);
  bulkEpUpdateSelToolbar();
}
async function refreshEditorPermList(){
  const listEl=document.getElementById("epPermList");
  if(!listEl) return;
  listEl.textContent="Memuat…";
  _epPermSelected=new Set();
  bulkEpUpdateSelToolbar();
  try{
    const {permissions}=await api("/api/forms/"+FORM_ID+"/editor-permissions");
    _epPermCache=permissions||[];

    if(!permissions||!permissions.length){
      listEl.innerHTML='<div class="muted" style="font-size:13px">Belum ada editor yang ditambahkan.</div>';
      return;
    }
    listEl.innerHTML=permissions.map(p=>{
      const filterCount=p.fieldFilters?Object.keys(p.fieldFilters).length:0;
      const filterSummary=filterCount?`· ${filterCount} filter variabel aktif`:"";
      return`<div style="border:1px solid var(--line);border-radius:8px;padding:10px 12px;margin-bottom:8px;display:flex;align-items:center;gap:10px;flex-wrap:wrap">
        <input type="checkbox" onchange="bulkEpToggleSel('${p.id}',this.checked)">
        <div style="flex:1;min-width:120px">
          <b>${esc(p.editorName||"(editor)")}</b>
          <div style="font-size:11px;color:var(--muted)">Akses kelola form aktif ${filterSummary}</div>
        </div>
        <div class="acts">
          <button class="btn" style="font-size:12px" onclick="openEpDetail('${p.id}','${esc(p.editorName||"editor")}')">Konfigurasi</button>
          <button class="btn danger" style="font-size:12px" onclick="removeEditorPerm('${p.id}','${esc(p.editorName||"editor")}')">Cabut</button>
        </div>
      </div>`;
    }).join("");
  }catch(e){
    listEl.textContent="Gagal: "+e.message;
  }
}

async function addEditorPermission(){
  const editorId=document.getElementById("epEditorSel")?.value||"";
  if(!editorId){adminToast("Pilih editor terlebih dahulu",true);return;}
  try{
    await api("/api/forms/"+FORM_ID+"/editor-permissions",{
      method:"POST",
      body:JSON.stringify({editorId})
    });
    document.getElementById("epEditorSel").value="";
    await refreshEditorPermList();
  }catch(e){
    adminToast("Gagal: "+e.message,true);
  }
}

async function removeEditorPerm(permId,name){
  adminConfirm(`Cabut akses editor "${name}" dari kuesioner ini?`,async()=>{
    try{
      await api("/api/editor-permissions/"+permId,{method:"DELETE"});
      await refreshEditorPermList();
    }catch(e){
      adminToast("Gagal: "+e.message,true);
    }
  });
}

/* ======================================================
   AKSES VIEWER (dulu viewerPermDlg)
   ====================================================== */

async function initViewerSection(){
  renderFilterChips("vpAddFilterList",{},"removeVpAddFilter");
  await refreshViewerList();
  buildFieldCheckboxes("vpAddFieldList",FORM_SCHEMA,[]);
  buildFieldOptions(FORM_SCHEMA,"vpAddFilterField");
  await refreshVpPermList();
}

async function refreshViewerList(){
  const el=document.getElementById("viewerList");
  el.textContent="Memuat…";
  try{
    const {viewers}=await api("/api/viewers");
    const sel=document.getElementById("vpViewerSel");
    if(sel){
      const cur=sel.value;
      sel.innerHTML=`<option value="">— pilih viewer —</option>`+
        viewers.map(v=>`<option value="${esc(v.id)}">${esc(v.username)}</option>`).join("");
      sel.value=cur;
    }
    if(!viewers.length){el.innerHTML='<div class="muted" style="font-size:13px">Belum ada viewer.</div>';return;}
    el.innerHTML=`<table style="width:100%;border-collapse:collapse;font-size:13px">
      <thead><tr style="background:var(--surface)">
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Username</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Email</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Catatan</th>
        <th style="text-align:left;padding:6px 8px;border-bottom:1px solid var(--line)">Status</th>
        <th style="padding:6px 8px;border-bottom:1px solid var(--line)"></th>
      </tr></thead>
      <tbody>${viewers.map(v=>`<tr>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2)">${esc(v.username)}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);color:var(--muted)">${esc(v.email||"—")}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);color:var(--muted)">${esc(v.note||"—")}</td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2)"><span class="tag ${v.isActive?"published":"archived"}">${v.isActive?"Aktif":"Nonaktif"}</span></td>
        <td style="padding:6px 8px;border-bottom:1px solid var(--line-2);text-align:right">
          <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteViewer('${v.id}','${esc(v.username)}')">Hapus</button>
        </td>
      </tr>`).join("")}</tbody>
    </table>`;
  }catch(e){el.textContent="Gagal: "+e.message;}
}

async function createViewer(){
  const email=(document.getElementById("vEmail")?.value||"").trim();
  const note=(document.getElementById("vNote")?.value||"").trim();
  if(!email){adminToast("Email Google wajib diisi",true);return;}
  try{
    await api("/api/viewers",{method:"POST",body:JSON.stringify({email,note})});
    if(document.getElementById("vEmail"))document.getElementById("vEmail").value="";
    if(document.getElementById("vNote"))document.getElementById("vNote").value="";
    await refreshViewerList();
  }catch(e){adminToast("Gagal: "+e.message,true);}
}

async function deleteViewer(id,name){
  adminConfirm(`Hapus viewer "${name}"? Semua akses kuesioner viewer ini akan ikut dihapus.`,async()=>{
    try{await api("/api/viewers/"+id,{method:"DELETE"});await refreshViewerList();}
    catch(e){adminToast("Gagal: "+e.message,true);}
  });
}

let _vpAddFilters={};
let _vpdFilters={};
let _epdPermId=null, _epdFilters={};

function renderFilterChips(containerId,filters,removeFn){
  const el=document.getElementById(containerId);
  if(!el)return;
  const entries=Object.entries(filters||{});
  if(!entries.length){
    el.innerHTML='<span style="font-size:11px;color:var(--muted)">Belum ada batasan filter.</span>';
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
  sel.innerHTML='<option value="">— variabel —</option>'+
    fields.map(f=>`<option value="${esc(f.name)}">${esc(f.label)}</option>`).join('');
  sel.value=cur;
}

function addVpAddFilter(){
  const field=document.getElementById("vpAddFilterField").value;
  const value=(document.getElementById("vpAddFilterValue").value||"").trim();
  if(!field||!value){adminToast("Pilih variabel dan masukkan nilai",true);return;}
  _vpAddFilters[field]=value;
  document.getElementById("vpAddFilterValue").value="";
  renderFilterChips("vpAddFilterList",_vpAddFilters,"removeVpAddFilter");
}
function removeVpAddFilter(field){
  delete _vpAddFilters[field];
  renderFilterChips("vpAddFilterList",_vpAddFilters,"removeVpAddFilter");
}

function addVpdFilter(){
  const field=document.getElementById("vpdFilterField").value;
  const value=(document.getElementById("vpdFilterValue").value||"").trim();
  if(!field||!value){adminToast("Pilih variabel dan masukkan nilai",true);return;}
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
  if(!field||!value){adminToast("Pilih variabel dan masukkan nilai",true);return;}
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
  try{
    const perm=await api("/api/editor-permissions/"+permId);
    _epdFilters=perm.fieldFilters||{};
    buildFieldOptions(FORM_SCHEMA,"epdFilterField");
    renderFilterChips("epdFilterList",_epdFilters,"removeEpdFilter");
    epDetailDlg.showModal();
  }catch(e){adminToast("Gagal memuat: "+e.message,true);}
}

async function saveEpDetail(){
  try{
    await api("/api/editor-permissions/"+_epdPermId,{
      method:"PUT",body:JSON.stringify({fieldFilters:_epdFilters})
    });
    epDetailDlg.close();
    await refreshEditorPermList();
  }catch(e){adminToast("Gagal menyimpan: "+e.message,true);}
}

let _vpPermCache=[];
let _vpPermSelected=new Set();
function bulkVpUpdateSelToolbar(){
  const n=_vpPermSelected.size;
  const mb=document.getElementById("vpManageSelBtn"),db=document.getElementById("vpDeleteSelBtn");
  if(mb){mb.disabled=n===0;mb.textContent=n?`Kelola Terpilih (${n})`:"Kelola Terpilih";}
  if(db){db.disabled=n===0;db.textContent=n?`Hapus Terpilih (${n})`:"Hapus Terpilih";}
}
function bulkVpToggleSel(id,checked){
  if(checked)_vpPermSelected.add(id);else _vpPermSelected.delete(id);
  bulkVpUpdateSelToolbar();
}
async function refreshVpPermList(){
  const el=document.getElementById("vpPermList");
  el.textContent="Memuat…";
  _vpPermSelected=new Set();
  bulkVpUpdateSelToolbar();
  try{
    const {permissions}=await api("/api/forms/"+FORM_ID+"/viewer-permissions");
    _vpPermCache=permissions||[];
    if(!permissions.length){el.innerHTML='<div class="muted" style="font-size:13px">Belum ada viewer yang ditambahkan.</div>';return;}
    el.innerHTML=permissions.map(p=>{
      const filterCount=p.fieldFilters?Object.keys(p.fieldFilters).length:0;
      return`<div style="border:1px solid var(--line);border-radius:8px;padding:10px 12px;margin-bottom:8px;display:flex;align-items:center;gap:10px;flex-wrap:wrap">
        <input type="checkbox" onchange="bulkVpToggleSel('${p.id}',this.checked)">
        <div style="flex:1;min-width:120px">
          <b>${esc(p.viewerUsername)}</b>
          <div style="font-size:11px;color:var(--muted)">
            ${p.respondentAccess==="all"?"Semua responden":`${p.allowedCount} responden dipilih`}
            · ${p.visibleFields&&p.visibleFields.length?p.visibleFields.length+" variabel":"Semua variabel"}
            ${filterCount?`· ${filterCount} filter variabel`:""}
          </div>
        </div>
        <div class="acts">
          <button class="btn" style="font-size:12px" onclick="openVpDetail('${p.id}','${esc(p.viewerUsername)}')">Konfigurasi</button>
          <button class="btn danger" style="font-size:12px" onclick="removeViewerPerm('${p.id}','${esc(p.viewerUsername)}')">Hapus</button>
        </div>
      </div>`;
    }).join("");
  }catch(e){el.textContent="Gagal: "+e.message;}
}

async function addViewerPermission(){
  const viewerId=document.getElementById("vpViewerSel").value;
  const respondentAccess=document.querySelector("input[name='vpRA']:checked")?.value||"all";
  if(!viewerId){adminToast("Pilih viewer terlebih dahulu",true);return;}
  const cbAll=[...document.querySelectorAll("#vpAddFieldList input[type=checkbox]")];
  const cbChecked=cbAll.filter(c=>c.checked).map(c=>c.value);
  const visibleFields=cbAll.length>0&&cbChecked.length<cbAll.length?cbChecked:[];
  try{
    await api("/api/forms/"+FORM_ID+"/viewer-permissions",{
      method:"POST",body:JSON.stringify({viewerId,respondentAccess,visibleFields,fieldFilters:_vpAddFilters})
    });
    document.getElementById("vpViewerSel").value="";
    document.querySelector("input[name='vpRA'][value='all']").checked=true;
    buildFieldCheckboxes("vpAddFieldList",FORM_SCHEMA,[]);
    _vpAddFilters={};
    renderFilterChips("vpAddFilterList",{},"removeVpAddFilter");
    await refreshVpPermList();
  }catch(e){adminToast("Gagal: "+e.message,true);}
}

async function removeViewerPerm(permId,viewerName){
  adminConfirm(`Cabut akses "${viewerName}" dari kuesioner ini?`,async()=>{
    try{await api("/api/viewer-permissions/"+permId,{method:"DELETE"});await refreshVpPermList();}
    catch(e){adminToast("Gagal: "+e.message,true);}
  });
}

let _vpdPermId=null;
async function openVpDetail(permId,viewerName){
  _vpdPermId=permId;
  document.getElementById("vpdViewerName").textContent=viewerName;

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
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (respondentsData.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");

    vpDetailDlg.showModal();
  }catch(e){adminToast("Gagal memuat: "+e.message,true);}
}

document.querySelectorAll("input[name='vpdRA']").forEach(rb=>{
  rb.addEventListener("change",()=>toggleRespondentSection(rb.value==="selected"));
});

function toggleRespondentSection(show){
  document.getElementById("vpdRespondentSection").style.display=show?"block":"none";
}

function renderAllowedRespondents(list){
  const el=document.getElementById("vpdRespondentList");
  if(!list.length){el.innerHTML='<div class="muted" style="font-size:11px">Belum ada responden dipilih.</div>';return;}
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
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Gagal: "+e.message,true);}
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
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){adminToast("Gagal: "+e.message,true);}
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
  if(!fields.length){el.innerHTML='<div style="font-size:12px;color:var(--muted)">Tidak ada variabel di kuesioner ini.</div>';return;}
  el.innerHTML=fields.map(f=>`
    <label style="display:flex;align-items:center;gap:8px;padding:4px 0;font-size:12px;cursor:pointer">
      <input type="checkbox" value="${esc(f.name)}" ${!checked.length||checked.includes(f.name)?"checked":""}>
      <span>${esc(f.label)}</span>
    </label>`).join("");
}

function vpCheckAll(on){
  document.querySelectorAll("#vpAddFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
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
    await api("/api/viewer-permissions/"+_vpdPermId,{
      method:"PUT",body:JSON.stringify({respondentAccess,visibleFields,fieldFilters:_vpdFilters})
    });
    vpDetailDlg.close();
    await refreshVpPermList();
  }catch(e){adminToast("Gagal menyimpan: "+e.message,true);}
}

/* ======================================================
   BULK ASSIGN — tambah/kelola massal viewer & editor
   ====================================================== */

function bulkParseLines(text){
  const lines=String(text||"").split(/\r?\n/).map(l=>l.trim()).filter(l=>l.length);
  if(!lines.length)return[];
  const delim=lines[0].includes("\t")?"\t":(lines[0].includes(",")?",":";");
  let rows=lines.map(l=>l.split(delim).map(c=>c.trim()));
  if(rows.length>1&&rows[0][0]&&!rows[0][0].includes("@")&&rows[1][0]&&rows[1][0].includes("@"))rows=rows.slice(1);
  return rows.map(c=>({email:(c[0]||"").toLowerCase(),note:c[1]||"",filterValue:c[2]||"",clientError:null,serverError:null,status:null}));
}

function bulkValidateRows(rows){
  const seen=new Map();
  rows.forEach(r=>{
    r.clientError=null;
    if(r.status==="created"||r.status==="updated")return;
    if(!r.email){r.clientError="Email wajib diisi";return;}
    if(!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(r.email)){r.clientError="Format email tidak valid";return;}
    if(seen.has(r.email)){r.clientError="Email duplikat pada baris ini";seen.get(r.email).clientError="Email duplikat pada baris ini";return;}
    seen.set(r.email,r);
  });
  return rows.every(r=>!r.clientError);
}

function bulkRowStatusHtml(r){
  if(r.status==="created")return"<span style='color:var(--ok)'>✓ Dibuat</span>";
  if(r.status==="updated")return"<span style='color:var(--ok)'>✓ Diperbarui</span>";
  const msg=r.status==="error"?r.serverError:r.clientError;
  return msg?`<span style="color:#b91c1c">${esc(msg)}</span>`:"";
}

function bulkFindFieldNode(schema,fieldName){
  let found=null;
  function walk(comps){
    for(const c of comps||[]){
      if(found)return;
      if(c.kind==="field"&&c.name===fieldName){found=c;return;}
      if(c.components)walk(c.components);
    }
  }
  for(const p of schema?.pages||[])walk(p.components||[]);
  return found;
}

function bulkFieldChoices(schema,fieldName){
  const f=bulkFindFieldNode(schema,fieldName);
  if(!f)return null;
  if(f.optionsRef){
    const tbl=schema.referenceData&&schema.referenceData[f.optionsRef];
    if(!tbl||tbl.source==="api"||!tbl.items)return null;
    return tbl.items.map(it=>({value:String(it.code),label:it.label!=null?String(it.label):String(it.code)}));
  }
  if(Array.isArray(f.options)&&f.options.length){
    return f.options.map(o=>({value:String(o.value),label:(typeof o.label==="string"?o.label:(o.label&&o.label.id))||String(o.value)}));
  }
  return null;
}

function bulkCoverageSummary(choices,filterField,rows,existingPerms){
  if(!choices||!filterField)return"";
  const counts=new Map(choices.map(c=>[c.value,0]));
  const bump=v=>{if(v==null||v==="")return;const k=String(v);if(counts.has(k))counts.set(k,counts.get(k)+1);};
  (existingPerms||[]).forEach(p=>{if(p.fieldFilters&&p.fieldFilters[filterField]!=null)bump(p.fieldFilters[filterField]);});
  (rows||[]).forEach(r=>{if(r.status!=="error")bump(r.filterValue);});
  const labelOf=v=>{const c=choices.find(x=>x.value===v);return c?c.label:v;};
  const zero=[...counts.entries()].filter(([,n])=>n===0).map(([v])=>labelOf(v));
  const dup=[...counts.entries()].filter(([,n])=>n>1).map(([v])=>labelOf(v));
  const parts=[];
  if(zero.length)parts.push(`${zero.length} belum ada petugas (${zero.slice(0,5).join(", ")}${zero.length>5?"…":""})`);
  if(dup.length)parts.push(`${dup.length} diisi >1 petugas (${dup.slice(0,5).join(", ")}${dup.length>5?"…":""})`);
  return parts.length?" · ⚠ "+parts.join(" · "):"";
}

async function bulkFillSuggestions(formId,fieldName,datalistId){
  const dl=document.getElementById(datalistId);
  if(!dl)return;
  dl.innerHTML="";
  if(!fieldName)return;
  try{
    const{values}=await api("/api/forms/"+formId+"/fields/"+encodeURIComponent(fieldName)+"/suggested-values");
    dl.innerHTML=(values||[]).map(v=>`<option value="${esc(v)}">`).join("");
  }catch(_){}
}

// ---- Viewer ----
let _bvRows=[];
function openBulkViewer(initialRows){
  _bvRows=initialRows||[];
  document.getElementById("bvPaste").value="";
  buildFieldOptions(FORM_SCHEMA,"bvFilterField");
  document.getElementById("bvFilterField").value="";
  document.getElementById("bvSuggest").innerHTML="";
  buildFieldCheckboxes("bvFieldList",FORM_SCHEMA,[]);
  const raAll=document.querySelector("input[name='bvRA'][value='all']");
  if(raAll)raAll.checked=true;
  document.getElementById("bvPreviewBox").style.display="none";
  document.getElementById("bvApplyBtn").disabled=true;
  bulkVpRenderTable();
  bulkViewerDlg.showModal();
  api("/api/forms").then(({forms})=>{
    const sel=document.getElementById("bvCopyFormSel");
    sel.innerHTML='<option value="">— pilih kuesioner —</option>'+
      (forms||[]).filter(f=>f.id!==FORM_ID).map(f=>`<option value="${esc(f.id)}">${esc(f.title)}</option>`).join("");
  }).catch(()=>{});
}
async function bulkVpCopyFromForm(){
  const otherId=document.getElementById("bvCopyFormSel").value;
  if(!otherId){adminToast("Pilih kuesioner sumber dulu",true);return;}
  try{
    const{permissions}=await api("/api/forms/"+otherId+"/viewer-permissions");
    const rows=(permissions||[]).map(p=>({
      email:p.viewerUsername,note:"",filterValue:"",clientError:null,serverError:null,status:null,sourceFieldFilters:p.fieldFilters||{},
    }));
    if(!rows.length){adminToast("Kuesioner sumber belum punya viewer",true);return;}
    _bvRows=_bvRows.concat(rows);
    bulkVpFilterFieldChanged();
  }catch(e){adminToast("Gagal menyalin: "+e.message,true);}
}
function bulkVpParse(){
  const parsed=bulkParseLines(document.getElementById("bvPaste").value);
  if(!parsed.length){adminToast("Tidak ada baris yang bisa diurai",true);return;}
  _bvRows=_bvRows.concat(parsed);
  document.getElementById("bvPaste").value="";
  bulkVpRenderTable();
}
function bulkVpAddRow(){_bvRows.push({email:"",note:"",filterValue:"",clientError:null,serverError:null,status:null});bulkVpRenderTable();}
function bulkVpRemoveRow(i){_bvRows.splice(i,1);bulkVpRenderTable();}
function bulkVpCellChange(i,field,value){if(_bvRows[i])_bvRows[i][field]=value;}
function bulkVpCheckAll(on){document.querySelectorAll("#bvFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});}

function bulkVpFilterFieldChanged(){
  const field=document.getElementById("bvFilterField").value;
  _bvRows.forEach(r=>{
    if(r.status==="created"||r.status==="updated")return;
    if(r.sourcePermId){
      const src=_vpPermCache.find(p=>p.id===r.sourcePermId);
      r.filterValue=(field&&src&&src.fieldFilters&&src.fieldFilters[field]!=null)?String(src.fieldFilters[field]):"";
    }else if(r.sourceFieldFilters){
      r.filterValue=(field&&r.sourceFieldFilters[field]!=null)?String(r.sourceFieldFilters[field]):"";
    }
  });
  if(!bulkFieldChoices(FORM_SCHEMA,field))bulkFillSuggestions(FORM_ID,field,"bvSuggest");
  bulkVpRenderTable();
}

function bulkVpManageSelected(){
  if(!_vpPermSelected.size)return;
  const rows=_vpPermCache.filter(p=>_vpPermSelected.has(p.id)).map(p=>({
    email:p.viewerUsername,note:"",filterValue:"",clientError:null,serverError:null,status:null,sourcePermId:p.id,
  }));
  openBulkViewer(rows);
}

function bulkVpDeleteSelected(){
  if(!_vpPermSelected.size)return;
  const ids=[..._vpPermSelected];
  adminConfirm(`Hapus akses ${ids.length} viewer terpilih dari kuesioner ini?`,async()=>{
    try{
      await api("/api/forms/"+FORM_ID+"/viewer-permissions/bulk",{method:"DELETE",body:JSON.stringify({permissionIds:ids})});
      await refreshVpPermList();
      adminToast(`${ids.length} akses viewer dihapus`);
    }catch(e){adminToast("Gagal: "+e.message,true);}
  });
}

async function downloadViewerPermsCsv(){
  try{
    const r=await fetch("/api/forms/"+FORM_ID+"/viewer-permissions.csv",{headers:H});
    if(!r.ok)throw new Error("HTTP "+r.status);
    const blob=await r.blob();const url=URL.createObjectURL(blob);
    const a=document.createElement("a");a.href=url;a.download="viewer-permissions-"+FORM_ID+".csv";a.click();URL.revokeObjectURL(url);
  }catch(e){adminToast("Gagal unduh: "+e.message,true);}
}

function bulkVpFilterCellHtml(r,i,choices,done){
  if(choices){
    return`<select ${done?"disabled":""} style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" onchange="bulkVpCellChange(${i},'filterValue',this.value)">
      <option value="">— pilih —</option>
      ${choices.map(c=>`<option value="${esc(c.value)}"${r.filterValue===c.value?" selected":""}>${esc(c.label)}</option>`).join("")}
    </select>`;
  }
  return`<input ${done?"disabled":""} value="${esc(r.filterValue)}" list="bvSuggest" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkVpCellChange(${i},'filterValue',this.value)">`;
}

function bulkVpRenderTable(){
  bulkValidateRows(_bvRows);
  document.getElementById("bvPreviewBox").style.display="none";
  document.getElementById("bvApplyBtn").disabled=true;
  const filterField=document.getElementById("bvFilterField").value;
  const choices=bulkFieldChoices(FORM_SCHEMA,filterField);
  const tbody=document.getElementById("bvRows");
  if(!_bvRows.length){
    tbody.innerHTML='<tr><td colspan="5" style="padding:14px;text-align:center;color:var(--muted)">Belum ada baris. Tempel daftar di atas lalu klik "Urai".</td></tr>';
  }else{
    tbody.innerHTML=_bvRows.map((r,i)=>{
      const done=r.status==="created"||r.status==="updated";
      const hasErr=r.clientError||r.status==="error";
      return`<tr style="${hasErr?"background:#fef2f2":""}">
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)"><input ${done?"disabled":""} value="${esc(r.email)}" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkVpCellChange(${i},'email',this.value)"></td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)"><input ${done?"disabled":""} value="${esc(r.note)}" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkVpCellChange(${i},'note',this.value)"></td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)">${bulkVpFilterCellHtml(r,i,choices,done)}</td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2);font-size:11px">${bulkRowStatusHtml(r)}</td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)">${done?"":`<button class="btn danger" style="font-size:11px;padding:2px 7px" type="button" onclick="bulkVpRemoveRow(${i})">✕</button>`}</td>
      </tr>`;
    }).join("");
  }
  const total=_bvRows.length,doneCount=_bvRows.filter(r=>r.status==="created"||r.status==="updated").length,errCount=_bvRows.filter(r=>r.clientError||r.status==="error").length;
  const coverage=bulkCoverageSummary(choices,filterField,_bvRows,_vpPermCache);
  document.getElementById("bvSummary").textContent=total?`${total} baris`+(doneCount?` · ${doneCount} sudah diterapkan`:"")+(errCount?` · ${errCount} error`:"")+coverage:coverage;
}

function bulkVpPreview(){
  if(!_bvRows.length){adminToast("Belum ada baris",true);return;}
  const ok=bulkValidateRows(_bvRows);
  bulkVpRenderTable();
  const box=document.getElementById("bvPreviewBox");
  const pending=_bvRows.filter(r=>r.status!=="created"&&r.status!=="updated");
  box.style.display="block";
  if(!ok){
    box.innerHTML='<b style="color:#b91c1c">Masih ada baris bermasalah.</b> Perbaiki dulu sebelum menerapkan.';
    document.getElementById("bvApplyBtn").disabled=true;
    return;
  }
  box.innerHTML=`Siap menerapkan <b>${pending.length}</b> baris ke kuesioner ini. Akun baru akan dibuat otomatis untuk email yang belum terdaftar sebagai viewer.`;
  document.getElementById("bvApplyBtn").disabled=pending.length===0;
}

async function bulkVpApply(){
  const respondentAccess=document.querySelector("input[name='bvRA']:checked")?.value||"all";
  const checked=[...document.querySelectorAll("#bvFieldList input:checked")].map(cb=>cb.value);
  const totalFields=document.querySelectorAll("#bvFieldList input").length;
  const visibleFields=checked.length===totalFields?[]:checked;
  const filterField=document.getElementById("bvFilterField").value;
  const pendingIdx=[],items=[];
  _bvRows.forEach((r,i)=>{
    if(r.status==="created"||r.status==="updated")return;
    pendingIdx.push(i);
    items.push({email:r.email,note:r.note,respondentAccess,visibleFields,fieldFilters:filterField&&r.filterValue?{[filterField]:r.filterValue}:{}});
  });
  if(!items.length)return;
  const btn=document.getElementById("bvApplyBtn");
  btn.disabled=true;btn.textContent="Menerapkan…";
  try{
    const{results}=await api("/api/forms/"+FORM_ID+"/viewer-permissions/bulk",{method:"POST",body:JSON.stringify({items})});
    results.forEach(res=>{
      const row=_bvRows[pendingIdx[res.index]];
      if(!row)return;
      row.status=res.status;
      row.serverError=res.status==="error"?res.error:null;
    });
    bulkVpRenderTable();
    await refreshVpPermList();
    await refreshViewerList();
    const okCount=results.filter(x=>x.status!=="error").length,errCount=results.length-okCount;
    adminToast(errCount?`${okCount} berhasil, ${errCount} gagal — perbaiki baris merah lalu terapkan lagi`:`${okCount} berhasil diterapkan`,!!errCount);
  }catch(e){adminToast("Gagal: "+e.message,true);}
  finally{btn.textContent="Terapkan";btn.disabled=_bvRows.every(r=>r.status==="created"||r.status==="updated");}
}

// ---- Editor ----
let _beRows=[];
function openBulkEditor(initialRows){
  _beRows=initialRows||[];
  document.getElementById("bePaste").value="";
  buildFieldOptions(FORM_SCHEMA,"beFilterField");
  document.getElementById("beFilterField").value="";
  document.getElementById("beSuggest").innerHTML="";
  document.getElementById("bePreviewBox").style.display="none";
  document.getElementById("beApplyBtn").disabled=true;
  bulkEpRenderTable();
  bulkEditorDlg.showModal();
  api("/api/forms").then(({forms})=>{
    const sel=document.getElementById("beCopyFormSel");
    sel.innerHTML='<option value="">— pilih kuesioner —</option>'+
      (forms||[]).filter(f=>f.id!==FORM_ID).map(f=>`<option value="${esc(f.id)}">${esc(f.title)}</option>`).join("");
  }).catch(()=>{});
}
async function bulkEpCopyFromForm(){
  const otherId=document.getElementById("beCopyFormSel").value;
  if(!otherId){adminToast("Pilih kuesioner sumber dulu",true);return;}
  try{
    const{permissions}=await api("/api/forms/"+otherId+"/editor-permissions");
    const rows=(permissions||[]).map(p=>({
      email:p.editorName,note:"",filterValue:"",clientError:null,serverError:null,status:null,sourceFieldFilters:p.fieldFilters||{},
    }));
    if(!rows.length){adminToast("Kuesioner sumber belum punya editor",true);return;}
    _beRows=_beRows.concat(rows);
    bulkEpFilterFieldChanged();
  }catch(e){adminToast("Gagal menyalin: "+e.message,true);}
}
function bulkEpParse(){
  const parsed=bulkParseLines(document.getElementById("bePaste").value);
  if(!parsed.length){adminToast("Tidak ada baris yang bisa diurai",true);return;}
  _beRows=_beRows.concat(parsed);
  document.getElementById("bePaste").value="";
  bulkEpRenderTable();
}
function bulkEpAddRow(){_beRows.push({email:"",note:"",filterValue:"",clientError:null,serverError:null,status:null});bulkEpRenderTable();}
function bulkEpRemoveRow(i){_beRows.splice(i,1);bulkEpRenderTable();}
function bulkEpCellChange(i,field,value){if(_beRows[i])_beRows[i][field]=value;}

function bulkEpFilterFieldChanged(){
  const field=document.getElementById("beFilterField").value;
  _beRows.forEach(r=>{
    if(r.status==="created"||r.status==="updated")return;
    if(r.sourcePermId){
      const src=_epPermCache.find(p=>p.id===r.sourcePermId);
      r.filterValue=(field&&src&&src.fieldFilters&&src.fieldFilters[field]!=null)?String(src.fieldFilters[field]):"";
    }else if(r.sourceFieldFilters){
      r.filterValue=(field&&r.sourceFieldFilters[field]!=null)?String(r.sourceFieldFilters[field]):"";
    }
  });
  if(!bulkFieldChoices(FORM_SCHEMA,field))bulkFillSuggestions(FORM_ID,field,"beSuggest");
  bulkEpRenderTable();
}

function bulkEpManageSelected(){
  if(!_epPermSelected.size)return;
  const rows=_epPermCache.filter(p=>_epPermSelected.has(p.id)).map(p=>({
    email:p.editorName,note:"",filterValue:"",clientError:null,serverError:null,status:null,sourcePermId:p.id,
  }));
  openBulkEditor(rows);
}

function bulkEpDeleteSelected(){
  if(!_epPermSelected.size)return;
  const ids=[..._epPermSelected];
  adminConfirm(`Cabut akses ${ids.length} editor terpilih dari kuesioner ini?`,async()=>{
    try{
      await api("/api/forms/"+FORM_ID+"/editor-permissions/bulk",{method:"DELETE",body:JSON.stringify({permissionIds:ids})});
      await refreshEditorPermList();
      adminToast(`${ids.length} akses editor dicabut`);
    }catch(e){adminToast("Gagal: "+e.message,true);}
  });
}

async function downloadEditorPermsCsv(){
  try{
    const r=await fetch("/api/forms/"+FORM_ID+"/editor-permissions.csv",{headers:H});
    if(!r.ok)throw new Error("HTTP "+r.status);
    const blob=await r.blob();const url=URL.createObjectURL(blob);
    const a=document.createElement("a");a.href=url;a.download="editor-permissions-"+FORM_ID+".csv";a.click();URL.revokeObjectURL(url);
  }catch(e){adminToast("Gagal unduh: "+e.message,true);}
}

function bulkEpFilterCellHtml(r,i,choices,done){
  if(choices){
    return`<select ${done?"disabled":""} style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" onchange="bulkEpCellChange(${i},'filterValue',this.value)">
      <option value="">— pilih —</option>
      ${choices.map(c=>`<option value="${esc(c.value)}"${r.filterValue===c.value?" selected":""}>${esc(c.label)}</option>`).join("")}
    </select>`;
  }
  return`<input ${done?"disabled":""} value="${esc(r.filterValue)}" list="beSuggest" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkEpCellChange(${i},'filterValue',this.value)">`;
}

function bulkEpRenderTable(){
  bulkValidateRows(_beRows);
  document.getElementById("bePreviewBox").style.display="none";
  document.getElementById("beApplyBtn").disabled=true;
  const filterField=document.getElementById("beFilterField").value;
  const choices=bulkFieldChoices(FORM_SCHEMA,filterField);
  const tbody=document.getElementById("beRows");
  if(!_beRows.length){
    tbody.innerHTML='<tr><td colspan="5" style="padding:14px;text-align:center;color:var(--muted)">Belum ada baris. Tempel daftar di atas lalu klik "Urai".</td></tr>';
  }else{
    tbody.innerHTML=_beRows.map((r,i)=>{
      const done=r.status==="created"||r.status==="updated";
      const hasErr=r.clientError||r.status==="error";
      return`<tr style="${hasErr?"background:#fef2f2":""}">
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)"><input ${done?"disabled":""} value="${esc(r.email)}" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkEpCellChange(${i},'email',this.value)"></td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)"><input ${done?"disabled":""} value="${esc(r.note)}" style="width:100%;font-size:12px;padding:3px 5px;border:1px solid var(--line);border-radius:4px" oninput="bulkEpCellChange(${i},'note',this.value)"></td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)">${bulkEpFilterCellHtml(r,i,choices,done)}</td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2);font-size:11px">${bulkRowStatusHtml(r)}</td>
        <td style="padding:4px 6px;border-bottom:1px solid var(--line-2)">${done?"":`<button class="btn danger" style="font-size:11px;padding:2px 7px" type="button" onclick="bulkEpRemoveRow(${i})">✕</button>`}</td>
      </tr>`;
    }).join("");
  }
  const total=_beRows.length,doneCount=_beRows.filter(r=>r.status==="created"||r.status==="updated").length,errCount=_beRows.filter(r=>r.clientError||r.status==="error").length;
  const coverage=bulkCoverageSummary(choices,filterField,_beRows,_epPermCache);
  document.getElementById("beSummary").textContent=total?`${total} baris`+(doneCount?` · ${doneCount} sudah diterapkan`:"")+(errCount?` · ${errCount} error`:"")+coverage:coverage;
}

function bulkEpPreview(){
  if(!_beRows.length){adminToast("Belum ada baris",true);return;}
  const ok=bulkValidateRows(_beRows);
  bulkEpRenderTable();
  const box=document.getElementById("bePreviewBox");
  const pending=_beRows.filter(r=>r.status!=="created"&&r.status!=="updated");
  box.style.display="block";
  if(!ok){
    box.innerHTML='<b style="color:#b91c1c">Masih ada baris bermasalah.</b> Perbaiki dulu sebelum menerapkan.';
    document.getElementById("beApplyBtn").disabled=true;
    return;
  }
  box.innerHTML=`Siap menerapkan <b>${pending.length}</b> baris ke kuesioner ini. Akun baru akan dibuat otomatis untuk email yang belum terdaftar sebagai editor.`;
  document.getElementById("beApplyBtn").disabled=pending.length===0;
}

async function bulkEpApply(){
  const filterField=document.getElementById("beFilterField").value;
  const pendingIdx=[],items=[];
  _beRows.forEach((r,i)=>{
    if(r.status==="created"||r.status==="updated")return;
    pendingIdx.push(i);
    items.push({email:r.email,note:r.note,fieldFilters:filterField&&r.filterValue?{[filterField]:r.filterValue}:{}});
  });
  if(!items.length)return;
  const btn=document.getElementById("beApplyBtn");
  btn.disabled=true;btn.textContent="Menerapkan…";
  try{
    const{results}=await api("/api/forms/"+FORM_ID+"/editor-permissions/bulk",{method:"POST",body:JSON.stringify({items})});
    results.forEach(res=>{
      const row=_beRows[pendingIdx[res.index]];
      if(!row)return;
      row.status=res.status;
      row.serverError=res.status==="error"?res.error:null;
    });
    bulkEpRenderTable();
    await refreshEditorPermList();
    await refreshEditorList();
    const okCount=results.filter(x=>x.status!=="error").length,errCount=results.length-okCount;
    adminToast(errCount?`${okCount} berhasil, ${errCount} gagal — perbaiki baris merah lalu terapkan lagi`:`${okCount} berhasil diterapkan`,!!errCount);
  }catch(e){adminToast("Gagal: "+e.message,true);}
  finally{btn.textContent="Terapkan";btn.disabled=_beRows.every(r=>r.status==="created"||r.status==="updated");}
}

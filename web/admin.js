const token=localStorage.getItem("eform_token");
if(!token) location.replace("/login");
const H={"Authorization":"Bearer "+token,"Content-Type":"application/json"};
const $=s=>document.querySelector(s);
const esc=s=>String(s??"").replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));

async function api(path,opts={}){
  const r=await fetch(path,{...opts,headers:{...H,...(opts.headers||{})}});
  if(r.status===401){localStorage.removeItem("eform_token");location.replace("/login");throw new Error("sesi habis");}
  const ct=r.headers.get("content-type")||""; const data=ct.includes("json")?await r.json():null;
  if(!r.ok) throw new Error((data&&data.error)||("HTTP "+r.status));
  return data;
}

let MY_ROLE="admin";
let MY_ID="";
let ACTIVE_TAB="forms";
let ACTIVE_USER_SUBTAB="viewer";
(async()=>{
  try{
    const me=await api("/api/auth/me");
    MY_ROLE=me.role||"admin";
    MY_ID=me.id||"";
    $("#who").textContent=me.username+" · "+me.role;
    setupAdminMenu();
  }catch(e){}
  load();
})();

function setupAdminMenu(){
  const tabUsersBtn=$("#tabUsersBtn");
  const btnNewForm=$("#btnNewForm");
  const canManageUsers=MY_ROLE==="superadmin"||MY_ROLE==="admin";
  if(tabUsersBtn) tabUsersBtn.hidden=!canManageUsers;
  if(btnNewForm) btnNewForm.style.display=canManageUsers?"":"none";

  // Sub-tab Admin: hanya superadmin
  const subtabAdminBtn=$("#subtabAdminBtn");
  if(subtabAdminBtn) subtabAdminBtn.hidden=MY_ROLE!=="superadmin";

  // Default sub-tab berdasar role
  ACTIVE_USER_SUBTAB=MY_ROLE==="superadmin"?"admin":"viewer";

  if(!canManageUsers){ switchTab("forms"); return; }

  $("#tabFormsBtn")?.addEventListener("click",()=>switchTab("forms"));
  tabUsersBtn?.addEventListener("click",()=>switchTab("users"));
  $("#refreshUsers")?.addEventListener("click",()=>{
    if(ACTIVE_USER_SUBTAB==="admin") loadUsers();
    else if(ACTIVE_USER_SUBTAB==="viewer") loadViewersTab();
    else if(ACTIVE_USER_SUBTAB==="editor") loadEditorsTab();
  });
  if(MY_ROLE==="superadmin") $("#btnCreateUser")?.addEventListener("click",createUserFromPanel);
  $("#btnCreateViewerTab")?.addEventListener("click",createViewerFromTab);
  $("#btnCreateEditorTab")?.addEventListener("click",createEditorFromTab);
  $("#subtabAdminBtn")?.addEventListener("click",()=>switchUserSubTab("admin"));
  $("#subtabViewerBtn")?.addEventListener("click",()=>switchUserSubTab("viewer"));
  $("#subtabEditorBtn")?.addEventListener("click",()=>switchUserSubTab("editor"));
}

function switchTab(tab){
  ACTIVE_TAB=tab;
  const formsTab=$("#tabFormsBtn");
  const usersTab=$("#tabUsersBtn");
  const formsSection=$("#formsSection");
  const usersSection=$("#usersSection");
  const newFormBtn=$("#btnNewForm");
  const isUsers=tab==="users";
  const canCreateForm=MY_ROLE==="admin"||MY_ROLE==="superadmin";
  formsSection.hidden=isUsers;
  usersSection.hidden=!isUsers;
  formsTab?.classList.toggle("active",!isUsers);
  usersTab?.classList.toggle("active",isUsers);
  if(newFormBtn) newFormBtn.style.display=(!isUsers&&canCreateForm)?"":"none";
  if(isUsers){
    switchUserSubTab(ACTIVE_USER_SUBTAB);
  }
}

async function load(){
  try{
    const {forms}=await api("/api/forms");
    const rows=$("#rows");
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    const colCount=canViewResults?5:4;
    if(!forms||!forms.length){rows.innerHTML=`<tr><td colspan="${colCount}" class="empty">Belum ada kuesioner. Klik “+ Kuesioner baru”.</td></tr>`;return;}
    const counts=canViewResults
      ? await Promise.all(forms.map(f=>api("/api/forms/"+f.id+"/responses?limit=1").then(d=>d.total).catch(()=>0)))
      : forms.map(()=>0);
    rows.innerHTML=forms.map((f,i)=>`<tr>
      <td><b>${esc(f.title)}</b><div class="muted">${esc(f.slug)}</div></td>
      <td><span class="tag ${f.status}">${f.status}</span></td>
      <td class="muted">${new Date(f.updatedAt).toLocaleString("id-ID")}</td>
      ${canViewResults?`<td>${counts[i]} ${counts[i]?`· <a href="/api/forms/${f.id}/responses.csv" onclick="return dl(event,'${f.id}')">CSV</a>`:""}</td>`:""}
      <td><div class="acts">
        <button class="btn" onclick="location.href='/builder?id=${f.id}'">Buka</button>
        <button class="btn" onclick="togglePub('${f.id}','${f.status}')">${f.status==="published"?"Tarik":"Publikasikan"}</button>
        <button class="btn" onclick="openShare('${f.id}','${esc(f.title)}','${f.status}')">Bagikan</button>
        ${canViewResults?`<button class="btn" onclick="location.href='/responses?id=${f.id}'">Jawaban${counts[i]>0?` (${counts[i]})`:""}</button>`:""}
        ${(MY_ROLE==="superadmin"||MY_ROLE==="admin")?`<button class="btn" onclick="openEditorPerm('${f.id}','${esc(f.title)}')">Akses Editor</button>`:""}
        ${(MY_ROLE==="superadmin"||MY_ROLE==="admin")?`<button class="btn" onclick="openViewerPerm('${f.id}','${esc(f.title)}')">Akses Viewer</button>`:""}
        <button class="btn danger" onclick="del('${f.id}','${esc(f.title)}')" ${counts[i]>0?'disabled title="Tidak dapat dihapus karena sudah ada jawaban"':""}>Hapus</button>
      </div></td></tr>`).join("");
  }catch(e){
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    $("#rows").innerHTML=`<tr><td colspan="${canViewResults?5:4}" class="empty">${esc(e.message)}</td></tr>`;
  }
}

let _usersCache=[];

async function loadUsers(){
  if(MY_ROLE!=="superadmin") return;
  const rows=$("#userRows");
  if(!rows) return;
  rows.innerHTML='<tr><td colspan="6" class="empty">Memuat…</td></tr>';
  try{
    const {users}=await api("/api/users");
    _usersCache=users||[];
    _renderUsersTab();
  }catch(e){
    rows.innerHTML=`<tr><td colspan="6" class="empty">${esc(e.message)}</td></tr>`;
  }
}

function _renderUsersTab(){
  const rows=$("#userRows");
  if(!rows) return;
  if(!_usersCache.length){rows.innerHTML='<tr><td colspan="6" class="empty">Belum ada user.</td></tr>';return;}
  rows.innerHTML=_usersCache.map(u=>`<tr id="urow-${u.id}">
    <td><b>${esc(u.username||"-")}</b></td>
    <td class="muted">${esc(u.email||"-")}</td>
    <td><span class="tag">${esc(u.role||"-")}</span></td>
    <td><span class="tag ${u.isActive?"published":"archived"}">${u.isActive?"Aktif":"Nonaktif"}</span></td>
    <td class="muted">${u.createdAt?new Date(u.createdAt).toLocaleString("id-ID"):"-"}</td>
    <td style="text-align:right;white-space:nowrap">
      <button class="btn" style="font-size:12px;padding:3px 8px" onclick="editAdminUser('${u.id}')">Edit</button>
      <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteAdminUser('${u.id}','${esc(u.username)}')"${u.id===MY_ID?' disabled title="Tidak bisa menghapus akun sendiri"':""}>Hapus</button>
    </td>
  </tr>`).join("");
}

function editAdminUser(id){
  document.querySelectorAll("[id^='uedit-']").forEach(el=>el.remove());
  const u=_usersCache.find(x=>x.id===id);
  if(!u) return;
  const tr=document.createElement("tr");
  tr.id="uedit-"+id;
  tr.innerHTML=`<td colspan="6" style="padding:12px;background:var(--surface);border-top:2px solid var(--accent)">
    <div style="display:flex;flex-wrap:wrap;gap:8px;align-items:flex-end">
      <div style="flex:1;min-width:120px">
        <div style="font-size:11px;color:var(--muted);margin-bottom:3px">Username</div>
        <input id="ueu-${id}" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid var(--line);border-radius:6px" value="${esc(u.username||"")}">
      </div>
      <div style="flex:1;min-width:160px">
        <div style="font-size:11px;color:var(--muted);margin-bottom:3px">Email</div>
        <input id="uem-${id}" type="email" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid var(--line);border-radius:6px" value="${esc(u.email||"")}">
      </div>
      <div style="min-width:110px">
        <div style="font-size:11px;color:var(--muted);margin-bottom:3px">Role</div>
        <select id="uer-${id}" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid var(--line);border-radius:6px">
          <option value="admin"${u.role==="admin"?" selected":""}>admin</option>
          <option value="superadmin"${u.role==="superadmin"?" selected":""}>superadmin</option>
        </select>
      </div>
      <div style="flex:1;min-width:180px">
        <div style="font-size:11px;color:var(--muted);margin-bottom:3px">Password baru <span style="font-weight:normal">(kosongkan jika tidak diubah)</span></div>
        <input id="uepw-${id}" type="password" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid var(--line);border-radius:6px" placeholder="min. 6 karakter">
      </div>
      <div style="display:flex;gap:6px;flex-shrink:0">
        <button class="btn primary" style="font-size:13px" onclick="saveAdminUser('${id}')">Simpan</button>
        <button class="btn" style="font-size:13px" onclick="cancelAdminUserEdit('${id}')">Batal</button>
      </div>
    </div>
    <div id="uemsg-${id}" style="font-size:12px;color:#b91c1c;margin-top:6px"></div>
  </td>`;
  document.getElementById("urow-"+id)?.insertAdjacentElement("afterend",tr);
  document.getElementById("ueu-"+id)?.focus();
}

function cancelAdminUserEdit(id){
  document.getElementById("uedit-"+id)?.remove();
}

async function saveAdminUser(id){
  const username=(document.getElementById("ueu-"+id)?.value||"").trim();
  const email=(document.getElementById("uem-"+id)?.value||"").trim();
  const role=document.getElementById("uer-"+id)?.value||"admin";
  const password=(document.getElementById("uepw-"+id)?.value||"").trim();
  const msg=document.getElementById("uemsg-"+id);
  if(!username){if(msg)msg.textContent="Username wajib diisi.";return;}
  if(password&&password.length<6){if(msg)msg.textContent="Password minimal 6 karakter.";return;}
  if(msg)msg.textContent="";
  try{
    const body={username,email,role};
    if(password) body.password=password;
    await api("/api/users/"+id,{method:"PATCH",body:JSON.stringify(body)});
    const u=_usersCache.find(x=>x.id===id);
    if(u){u.username=username;u.email=email;u.role=role;}
    document.getElementById("uedit-"+id)?.remove();
    _renderUsersTab();
  }catch(e){if(msg)msg.textContent="Gagal: "+e.message;}
}

async function deleteAdminUser(id,name){
  if(id===MY_ID){alert("Tidak bisa menghapus akun sendiri.");return;}
  if(!confirm(`Hapus user "${name}"? Tindakan ini tidak bisa dibatalkan.`))return;
  try{
    await api("/api/users/"+id,{method:"DELETE"});
    await loadUsers();
  }catch(e){alert("Gagal: "+e.message);}
}

async function createUserFromPanel(){
  const username=(""+($("#uUsername")?.value||"")).trim();
  const email=(""+($("#uEmail")?.value||"")).trim();
  const password=(""+($("#uPassword")?.value||"")).trim();
  const role=(""+($("#uRole")?.value||"admin")).trim();
  const msg=$("#userMsg");

  if(!username){
    if(msg) msg.textContent="Username wajib diisi.";
    $("#uUsername")?.focus();
    return;
  }
  if(password.length<6){
    if(msg) msg.textContent="Password minimal 6 karakter.";
    $("#uPassword")?.focus();
    return;
  }

  const btn=$("#btnCreateUser");
  if(btn){btn.disabled=true;btn.textContent="Membuat…";}
  if(msg) msg.textContent="";
  try{
    await api("/api/users",{
      method:"POST",
      body:JSON.stringify({username,email,password,role})
    });
    if(msg) msg.textContent="User berhasil dibuat.";
    if($("#uUsername")) $("#uUsername").value="";
    if($("#uEmail")) $("#uEmail").value="";
    if($("#uPassword")) $("#uPassword").value="";
    if($("#uRole")) $("#uRole").value="admin";
    await loadUsers();
  }catch(e){
    if(msg) msg.textContent="Gagal: "+e.message;
  }finally{
    if(btn){btn.disabled=false;btn.textContent="+ Buat User";}
  }
}

async function dl(ev,id){ // unduh CSV dengan header auth
  ev.preventDefault();
  const r=await fetch("/api/forms/"+id+"/responses.csv",{headers:H});
  const blob=await r.blob(); const url=URL.createObjectURL(blob);
  const a=document.createElement("a"); a.href=url; a.download="responses-"+id+".csv"; a.click(); URL.revokeObjectURL(url);
  return false;
}
async function togglePub(id,status){
  const next=status==="published"?"draft":"published";
  try{await api("/api/forms/"+id+"/publish",{method:"POST",body:JSON.stringify({status:next})});load();}catch(e){alert(e.message);}
}
async function del(id,title){
  if(!confirm("Hapus kuesioner \""+title+"\" beserta semua jawabannya?"))return;
  try{await api("/api/forms/"+id,{method:"DELETE"});load();}catch(e){alert(e.message);}
}

let shareFormId=null;
async function openShare(id,title,status){
  shareFormId=id;
  $("#shareNote").innerHTML = status==="published"
    ? "Kuesioner sudah <b>published</b> — tautan bisa langsung diakses publik."
    : "⚠️ Kuesioner masih <b>draft</b>. Tautan dibuat, tapi publik baru bisa membuka setelah dipublikasikan.";
  // Reset form buat tautan
  $("#shareLabel").value="";$("#sharePw").value="";
  $("#shareMulti").checked=false;
  document.getElementById("shareAccessPublic").checked=true;
  pendingEmails=[];renderPendingEmails();
  $("#restrictedSection").style.display="none";
  $("#shareList").innerHTML="Memuat…"; shareDlg.showModal(); refreshShares();
}

// Toggle section email saat pilih mode akses
document.getElementById("shareAccessRestricted").addEventListener("change",()=>{
  $("#restrictedSection").style.display="block";
  $("#newEmailInput").focus();
});
document.getElementById("shareAccessPublic").addEventListener("change",()=>{
  $("#restrictedSection").style.display="none";
});

// ---- Daftar email sementara sebelum share dibuat ----
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
  if(pendingEmails.some(e=>e.email===email)){alert("Email sudah ada di daftar");return;}
  pendingEmails.push({email,note});
  $("#newEmailInput").value="";$("#newEmailNote").value="";
  $("#newEmailInput").focus();
  renderPendingEmails();
});
// Tekan Enter di input email langsung tambah
$("#newEmailInput").addEventListener("keydown",e=>{if(e.key==="Enter"){e.preventDefault();$("#btnAddNewEmail").click();}});

// ---- helper konversi ISO → nilai datetime-local input ----
function toLocalDT(iso){
  if(!iso)return"";
  const d=new Date(iso);
  const p=n=>String(n).padStart(2,"0");
  return`${d.getFullYear()}-${p(d.getMonth()+1)}-${p(d.getDate())}T${p(d.getHours())}:${p(d.getMinutes())}`;
}

// ---- state edit inline ----
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
  }catch(e){alert(e.message);if(btn){btn.disabled=false;btn.textContent="Simpan";}}
}

async function refreshShares(){
  try{
    const {shares}=await api("/api/forms/"+shareFormId+"/shares");
    if(!shares||!shares.length){$("#shareList").innerHTML="<div class='muted'>Belum ada tautan.</div>";return;}
    // Muat daftar email untuk share restricted secara paralel
    const emailMap={};
    await Promise.all(shares.filter(s=>s.accessMode==="restricted").map(async s=>{
      try{const {emails}=await api("/api/shares/"+s.id+"/allowed-emails");emailMap[s.id]=emails||[];}catch{emailMap[s.id]=[];}
    }));
    $("#shareList").innerHTML=shares.map(s=>{
      const isEditing=s.id===editingShareId;
      const badges=[];
      if(!s.isActive)badges.push("<span class='tag archived'>nonaktif</span>");
      if(s.hasPassword)badges.push("🔒");
      if(s.multiResponse)badges.push("<span class='tag'>multi-respons</span>");
      if(s.accessMode==="restricted")badges.push("<span class='tag'>terbatas</span>");

      // Form edit inline
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

      // Section email untuk share restricted (tampil di luar mode edit)
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

      return `<div class="share">
        <div style="display:flex;align-items:center;justify-content:space-between;gap:8px">
          <div><b>${esc(s.label||"(tanpa label)")}</b> ${badges.join(" ")} · ${s.viewCount}×</div>
          ${s.isActive&&!isEditing?`<button class="btn btn-xs" onclick="startEdit('${s.id}')">Edit</button>`:""}
        </div>
        <div style="margin:6px 0"><code>${esc(s.shareUrl)}</code></div>
        ${editSection}${emailSection}
        <div class="acts" style="margin-top:8px">
          <button class="btn" onclick="navigator.clipboard.writeText('${esc(s.shareUrl)}')">Salin</button>
          <a class="btn" href="${esc(s.shareUrl)}" target="_blank">Buka</a>
          ${!isEditing?(s.isActive?`<button class="btn danger" onclick="revoke('${s.id}')">Cabut</button>`:`<button class="btn danger" onclick="deleteShare('${s.id}')">Hapus</button>`):""}
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
  }catch(e){alert(e.message);}
}
async function removeEmail(id){
  try{await api("/api/share-emails/"+id,{method:"DELETE"});refreshShares();}catch(e){alert(e.message);}
}
async function revoke(id){ try{await api("/api/shares/"+id,{method:"DELETE"});refreshShares();}catch(e){alert(e.message);} }
async function deleteShare(id){
  if(!confirm("Hapus permanen tautan ini beserta semua konfigurasinya?"))return;
  try{await api("/api/shares/"+id+"/permanent",{method:"DELETE"});refreshShares();}catch(e){alert(e.message);}
}
$("#makeShare").addEventListener("click",async()=>{
  try{
    const accessMode=document.querySelector("input[name='shareAccess']:checked")?.value||"public";
    const sh=await api("/api/forms/"+shareFormId+"/shares",{method:"POST",body:JSON.stringify({
      label:$("#shareLabel").value.trim(),
      allowResponses:$("#shareAllow").checked,
      multiResponse:$("#shareMulti").checked,
      accessMode,
      password:$("#sharePw").value
    })});
    // Simpan email yang sudah disiapkan ke share baru
    if(accessMode==="restricted"&&pendingEmails.length){
      await Promise.all(pendingEmails.map(e=>
        api("/api/shares/"+sh.id+"/allowed-emails",{method:"POST",body:JSON.stringify(e)}).catch(()=>{})
      ));
    }
    pendingEmails=[];renderPendingEmails();
    $("#shareLabel").value="";$("#sharePw").value="";$("#shareMulti").checked=false;
    document.getElementById("shareAccessPublic").checked=true;
    $("#restrictedSection").style.display="none";
    refreshShares();
  }catch(e){alert(e.message);}
});

$("#logout").addEventListener("click",()=>{localStorage.removeItem("eform_token");localStorage.removeItem("eform_user");location.replace("/login");});
$("#refresh").addEventListener("click",()=>{
  if(ACTIVE_TAB==="users"){
    if(ACTIVE_USER_SUBTAB==="admin") loadUsers();
    else if(ACTIVE_USER_SUBTAB==="viewer") loadViewersTab();
    else if(ACTIVE_USER_SUBTAB==="editor") loadEditorsTab();
    return;
  }
  load();
});

/* ======================================================
   USER TAB — sub-tab switching
   ====================================================== */

function switchUserSubTab(tab){
  ACTIVE_USER_SUBTAB=tab;
  ["admin","viewer","editor"].forEach(s=>{
    const sec=document.getElementById(s+"SubSection");
    const btn=document.getElementById("subtab"+s[0].toUpperCase()+s.slice(1)+"Btn");
    if(sec) sec.hidden=s!==tab;
    if(btn) btn.classList.toggle("active",s===tab);
  });
  if(tab==="admin") loadUsers();
  else if(tab==="viewer") loadViewersTab();
  else if(tab==="editor") loadEditorsTab();
}

/* ======================================================
   USER TAB — viewer management
   ====================================================== */

let _viewersCache=[];

async function loadViewersTab(){
  const rows=document.getElementById("viewerTabRows");
  if(!rows) return;
  rows.innerHTML='<tr><td colspan="5" class="empty">Memuat…</td></tr>';
  try{
    const {viewers}=await api("/api/viewers");
    _viewersCache=viewers||[];
    _renderViewersTab();
  }catch(e){rows.innerHTML=`<tr><td colspan="5" class="empty">${esc(e.message)}</td></tr>`;}
}

function _renderViewersTab(){
  const rows=document.getElementById("viewerTabRows");
  if(!rows) return;
  if(!_viewersCache.length){rows.innerHTML='<tr><td colspan="5" class="empty">Belum ada viewer.</td></tr>';return;}
  rows.innerHTML=_viewersCache.map(v=>`<tr>
    <td><b>${esc(v.email||v.username||"-")}</b></td>
    <td id="vnote-${v.id}" class="muted">${v.note?esc(v.note):"—"}</td>
    <td><span class="tag ${v.isActive?"published":"archived"}">${v.isActive?"Aktif":"Nonaktif"}</span></td>
    <td class="muted">${v.createdAt?new Date(v.createdAt).toLocaleString("id-ID"):"-"}</td>
    <td style="text-align:right;white-space:nowrap" id="vact-${v.id}">
      <button class="btn" style="font-size:12px;padding:3px 8px" onclick="editViewerNote('${v.id}')">Edit</button>
      <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteViewerFromTab('${v.id}','${esc(v.email||v.username)}')">Hapus</button>
    </td>
  </tr>`).join("");
}

function editViewerNote(id){
  const v=_viewersCache.find(x=>x.id===id);
  if(!v) return;
  const noteCell=document.getElementById("vnote-"+id);
  const actCell=document.getElementById("vact-"+id);
  if(!noteCell||!actCell) return;
  noteCell.innerHTML=`<input id="vni-${id}" style="width:100%;font-size:13px;padding:3px 6px;border:1px solid var(--line);border-radius:4px" value="${esc(v.note||"")}">`;
  actCell.innerHTML=`<button class="btn primary" style="font-size:12px;padding:3px 8px" onclick="saveViewerNote('${id}')">Simpan</button>
    <button class="btn" style="font-size:12px;padding:3px 8px" onclick="_renderViewersTab()">Batal</button>`;
  document.getElementById("vni-"+id)?.focus();
}

async function saveViewerNote(id){
  const inp=document.getElementById("vni-"+id);
  if(!inp) return;
  const note=inp.value.trim();
  try{
    await api("/api/viewers/"+id,{method:"PATCH",body:JSON.stringify({note})});
    const v=_viewersCache.find(x=>x.id===id);
    if(v) v.note=note;
    _renderViewersTab();
  }catch(e){alert("Gagal menyimpan: "+e.message);}
}

async function createViewerFromTab(){
  const email=(document.getElementById("vtEmail")?.value||"").trim();
  const note=(document.getElementById("vtNote")?.value||"").trim();
  const msg=document.getElementById("viewerTabMsg");
  if(!email){if(msg)msg.textContent="Email wajib diisi.";document.getElementById("vtEmail")?.focus();return;}
  const btn=document.getElementById("btnCreateViewerTab");
  if(btn){btn.disabled=true;btn.textContent="Menambahkan…";}
  if(msg)msg.textContent="";
  try{
    await api("/api/viewers",{method:"POST",body:JSON.stringify({email,note})});
    if(msg)msg.textContent="Viewer berhasil ditambahkan.";
    if(document.getElementById("vtEmail"))document.getElementById("vtEmail").value="";
    if(document.getElementById("vtNote"))document.getElementById("vtNote").value="";
    await loadViewersTab();
  }catch(e){if(msg)msg.textContent="Gagal: "+e.message;}
  finally{if(btn){btn.disabled=false;btn.textContent="+ Tambah Viewer";}}
}

async function deleteViewerFromTab(id,name){
  if(!confirm(`Hapus viewer "${name}"? Semua akses kuesioner viewer ini akan ikut dihapus.`))return;
  try{await api("/api/viewers/"+id,{method:"DELETE"});await loadViewersTab();}
  catch(e){alert("Gagal: "+e.message);}
}

/* ======================================================
   USER TAB — editor management
   ====================================================== */

let _editorsCache=[];

async function loadEditorsTab(){
  const rows=document.getElementById("editorTabRows");
  if(!rows) return;
  rows.innerHTML='<tr><td colspan="5" class="empty">Memuat…</td></tr>';
  try{
    const {editors}=await api("/api/editors");
    _editorsCache=editors||[];
    _renderEditorsTab();
  }catch(e){rows.innerHTML=`<tr><td colspan="5" class="empty">${esc(e.message)}</td></tr>`;}
}

function _renderEditorsTab(){
  const rows=document.getElementById("editorTabRows");
  if(!rows) return;
  if(!_editorsCache.length){rows.innerHTML='<tr><td colspan="5" class="empty">Belum ada editor.</td></tr>';return;}
  rows.innerHTML=_editorsCache.map(e=>`<tr>
    <td><b>${esc(e.email||e.username||"-")}</b></td>
    <td id="enote-${e.id}" class="muted">${e.note?esc(e.note):"—"}</td>
    <td><span class="tag ${e.isActive?"published":"archived"}">${e.isActive?"Aktif":"Nonaktif"}</span></td>
    <td class="muted">${e.createdAt?new Date(e.createdAt).toLocaleString("id-ID"):"-"}</td>
    <td style="text-align:right;white-space:nowrap" id="eact-${e.id}">
      <button class="btn" style="font-size:12px;padding:3px 8px" onclick="editEditorNote('${e.id}')">Edit</button>
      <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteEditorFromTab('${e.id}','${esc(e.email||e.username)}')">Hapus</button>
    </td>
  </tr>`).join("");
}

function editEditorNote(id){
  const e=_editorsCache.find(x=>x.id===id);
  if(!e) return;
  const noteCell=document.getElementById("enote-"+id);
  const actCell=document.getElementById("eact-"+id);
  if(!noteCell||!actCell) return;
  noteCell.innerHTML=`<input id="eni-${id}" style="width:100%;font-size:13px;padding:3px 6px;border:1px solid var(--line);border-radius:4px" value="${esc(e.note||"")}">`;
  actCell.innerHTML=`<button class="btn primary" style="font-size:12px;padding:3px 8px" onclick="saveEditorNote('${id}')">Simpan</button>
    <button class="btn" style="font-size:12px;padding:3px 8px" onclick="_renderEditorsTab()">Batal</button>`;
  document.getElementById("eni-"+id)?.focus();
}

async function saveEditorNote(id){
  const inp=document.getElementById("eni-"+id);
  if(!inp) return;
  const note=inp.value.trim();
  try{
    await api("/api/editors/"+id,{method:"PATCH",body:JSON.stringify({note})});
    const e=_editorsCache.find(x=>x.id===id);
    if(e) e.note=note;
    _renderEditorsTab();
  }catch(e){alert("Gagal menyimpan: "+e.message);}
}

async function createEditorFromTab(){
  const email=(document.getElementById("etEmail")?.value||"").trim();
  const note=(document.getElementById("etNote")?.value||"").trim();
  const msg=document.getElementById("editorTabMsg");
  if(!email){if(msg)msg.textContent="Email wajib diisi.";document.getElementById("etEmail")?.focus();return;}
  const btn=document.getElementById("btnCreateEditorTab");
  if(btn){btn.disabled=true;btn.textContent="Menambahkan…";}
  if(msg)msg.textContent="";
  try{
    await api("/api/editors",{method:"POST",body:JSON.stringify({email,note})});
    if(msg)msg.textContent="Editor berhasil ditambahkan.";
    if(document.getElementById("etEmail"))document.getElementById("etEmail").value="";
    if(document.getElementById("etNote"))document.getElementById("etNote").value="";
    await loadEditorsTab();
  }catch(e){if(msg)msg.textContent="Gagal: "+e.message;}
  finally{if(btn){btn.disabled=false;btn.textContent="+ Tambah Editor";}}
}

async function deleteEditorFromTab(id,name){
  if(!confirm(`Hapus editor "${name}"? Semua akses kuesioner editor ini akan ikut dihapus.`))return;
  try{await api("/api/editors/"+id,{method:"DELETE"});await loadEditorsTab();}
  catch(e){alert("Gagal: "+e.message);}
}

/* ======================================================
   EDITOR MANAGEMENT (superadmin)
   ====================================================== */

let _epFormId=null, _epFormSchema=null;

async function openEditorPerm(formId,formTitle){
  _epFormId=formId;
  _epFormSchema=null;
  document.getElementById("epFormTitle").textContent=formTitle;
  editorPermDlg.showModal();
  await refreshEditorsAndPerms();
  try{
    const formData=await api("/api/forms/"+formId);
    _epFormSchema=formData.schema;
  }catch{}
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
    const {editors}=await api("/api/editors?formId="+encodeURIComponent(_epFormId||""));
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
  if(!email){alert("Email Google wajib diisi");return;}
  try{
    await api("/api/editors",{method:"POST",body:JSON.stringify({email,note})});
    if(document.getElementById("eEmail"))document.getElementById("eEmail").value="";
    if(document.getElementById("eNote"))document.getElementById("eNote").value="";
    await refreshEditorsAndPerms();
  }catch(e){
    alert("Gagal: "+e.message);
  }
}

async function deleteEditor(id,name){
  if(!confirm(`Hapus editor "${name}"? Semua akses form editor ini akan ikut dihapus.`))return;
  try{
    await api("/api/editors/"+id+"?formId="+encodeURIComponent(_epFormId||""),{method:"DELETE"});
    await refreshEditorsAndPerms();
  }catch(e){
    alert("Gagal: "+e.message);
  }
}

async function refreshEditorPermList(){
  const listEl=document.getElementById("epPermList");
  if(!listEl||!_epFormId) return;
  listEl.textContent="Memuat…";
  try{
    const {permissions}=await api("/api/forms/"+_epFormId+"/editor-permissions");

    if(!permissions||!permissions.length){
      listEl.innerHTML='<div class="muted" style="font-size:13px">Belum ada editor yang ditambahkan.</div>';
      return;
    }
    listEl.innerHTML=permissions.map(p=>{
      const filterCount=p.fieldFilters?Object.keys(p.fieldFilters).length:0;
      const filterSummary=filterCount?`· ${filterCount} filter variabel aktif`:"";
      return`<div style="border:1px solid var(--line);border-radius:8px;padding:10px 12px;margin-bottom:8px;display:flex;align-items:center;gap:10px;flex-wrap:wrap">
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
  if(!editorId){alert("Pilih editor terlebih dahulu");return;}
  try{
    await api("/api/forms/"+_epFormId+"/editor-permissions",{
      method:"POST",
      body:JSON.stringify({editorId})
    });
    document.getElementById("epEditorSel").value="";
    await refreshEditorPermList();
  }catch(e){
    alert("Gagal: "+e.message);
  }
}

async function removeEditorPerm(permId,name){
  if(!confirm(`Cabut akses editor "${name}" dari kuesioner ini?`)) return;
  try{
    await api("/api/editor-permissions/"+permId,{method:"DELETE"});
    await refreshEditorPermList();
  }catch(e){
    alert("Gagal: "+e.message);
  }
}

/* ======================================================
   VIEWER MANAGEMENT (superadmin)
   ====================================================== */

// --- Akun viewer (dikelola langsung dari dialog akses viewer per kuesioner) ---
async function refreshViewerList(){
  const el=document.getElementById("viewerList");
  el.textContent="Memuat…";
  try{
    const {viewers}=await api("/api/viewers?formId="+encodeURIComponent(_vpFormId||""));
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
  if(!email){alert("Email Google wajib diisi");return;}
  try{
    await api("/api/viewers",{method:"POST",body:JSON.stringify({email,note})});
    if(document.getElementById("vEmail"))document.getElementById("vEmail").value="";
    if(document.getElementById("vNote"))document.getElementById("vNote").value="";
    await refreshViewerList();
  }catch(e){alert("Gagal: "+e.message);}
}

async function deleteViewer(id,name){
  if(!confirm(`Hapus viewer "${name}"? Semua akses kuesioner viewer ini akan ikut dihapus.`))return;
  try{await api("/api/viewers/"+id+"?formId="+encodeURIComponent(_vpFormId||""),{method:"DELETE"});await refreshViewerList();}
  catch(e){alert("Gagal: "+e.message);}
}

// --- Field filter helpers (shared for viewer + editor) ---
let _vpAddFilters={};  // filter untuk dialog tambah viewer permission
let _vpdFilters={};    // filter untuk dialog edit detail viewer permission
let _epdPermId=null, _epdFilters={};  // filter untuk dialog detail editor permission

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

// Viewer Add dialog filters
function addVpAddFilter(){
  const field=document.getElementById("vpAddFilterField").value;
  const value=(document.getElementById("vpAddFilterValue").value||"").trim();
  if(!field||!value){alert("Pilih variabel dan masukkan nilai");return;}
  _vpAddFilters[field]=value;
  document.getElementById("vpAddFilterValue").value="";
  renderFilterChips("vpAddFilterList",_vpAddFilters,"removeVpAddFilter");
}
function removeVpAddFilter(field){
  delete _vpAddFilters[field];
  renderFilterChips("vpAddFilterList",_vpAddFilters,"removeVpAddFilter");
}

// Viewer Detail dialog filters
function addVpdFilter(){
  const field=document.getElementById("vpdFilterField").value;
  const value=(document.getElementById("vpdFilterValue").value||"").trim();
  if(!field||!value){alert("Pilih variabel dan masukkan nilai");return;}
  _vpdFilters[field]=value;
  document.getElementById("vpdFilterValue").value="";
  renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");
}
function removeVpdFilter(field){
  delete _vpdFilters[field];
  renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");
}

// Editor Detail dialog filters
function addEpdFilter(){
  const field=document.getElementById("epdFilterField").value;
  const value=(document.getElementById("epdFilterValue").value||"").trim();
  if(!field||!value){alert("Pilih variabel dan masukkan nilai");return;}
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
    buildFieldOptions(_epFormSchema,"epdFilterField");
    renderFilterChips("epdFilterList",_epdFilters,"removeEpdFilter");
    epDetailDlg.showModal();
  }catch(e){alert("Gagal memuat: "+e.message);}
}

async function saveEpDetail(){
  try{
    await api("/api/editor-permissions/"+_epdPermId,{
      method:"PUT",body:JSON.stringify({fieldFilters:_epdFilters})
    });
    epDetailDlg.close();
    await refreshEditorPermList();
  }catch(e){alert("Gagal menyimpan: "+e.message);}
}

// --- Dialog akses viewer per kuesioner ---
let _vpFormId=null, _vpFormSchema=null;
async function openViewerPerm(formId,formTitle){
  _vpFormId=formId;
  _vpAddFilters={};
  document.getElementById("vpFormTitle").textContent=formTitle;
  renderFilterChips("vpAddFilterList",{},"removeVpAddFilter");
  viewerPermDlg.showModal();
  await refreshViewerList();
  try{
    const formData=await api("/api/forms/"+formId);
    _vpFormSchema=formData.schema;
    buildFieldCheckboxes("vpAddFieldList",formData.schema,[]);
    buildFieldOptions(formData.schema,"vpAddFilterField");
  }catch{
    document.getElementById("vpAddFieldList").innerHTML=
      '<span style="font-size:12px;color:var(--muted)">Gagal memuat variabel.</span>';
  }
  await refreshVpPermList();
}

async function refreshVpPermList(){
  const el=document.getElementById("vpPermList");
  el.textContent="Memuat…";
  try{
    const {permissions}=await api("/api/forms/"+_vpFormId+"/viewer-permissions");
    if(!permissions.length){el.innerHTML='<div class="muted" style="font-size:13px">Belum ada viewer yang ditambahkan.</div>';return;}
    el.innerHTML=permissions.map(p=>{
      const filterCount=p.fieldFilters?Object.keys(p.fieldFilters).length:0;
      return`<div style="border:1px solid var(--line);border-radius:8px;padding:10px 12px;margin-bottom:8px;display:flex;align-items:center;gap:10px;flex-wrap:wrap">
        <div style="flex:1;min-width:120px">
          <b>${esc(p.viewerUsername)}</b>
          <div style="font-size:11px;color:var(--muted)">
            ${p.respondentAccess==="all"?"Semua responden":`${p.allowedCount} responden dipilih`}
            · ${p.visibleFields&&p.visibleFields.length?p.visibleFields.length+" variabel":"Semua variabel"}
            ${filterCount?`· ${filterCount} filter variabel`:""}
          </div>
        </div>
        <div class="acts">
          <button class="btn" style="font-size:12px" onclick="openVpDetail('${p.id}','${esc(p.viewerUsername)}','${p.formId}')">Konfigurasi</button>
          <button class="btn danger" style="font-size:12px" onclick="removeViewerPerm('${p.id}','${esc(p.viewerUsername)}')">Hapus</button>
        </div>
      </div>`;
    }).join("");
  }catch(e){el.textContent="Gagal: "+e.message;}
}

async function addViewerPermission(){
  const viewerId=document.getElementById("vpViewerSel").value;
  const respondentAccess=document.querySelector("input[name='vpRA']:checked")?.value||"all";
  if(!viewerId){alert("Pilih viewer terlebih dahulu");return;}
  // Baca field yang dicentang — jika semua tercentang kirim [] (semua terlihat)
  const cbAll=[...document.querySelectorAll("#vpAddFieldList input[type=checkbox]")];
  const cbChecked=cbAll.filter(c=>c.checked).map(c=>c.value);
  const visibleFields=cbAll.length>0&&cbChecked.length<cbAll.length?cbChecked:[];
  try{
    await api("/api/forms/"+_vpFormId+"/viewer-permissions",{
      method:"POST",body:JSON.stringify({viewerId,respondentAccess,visibleFields,fieldFilters:_vpAddFilters})
    });
    document.getElementById("vpViewerSel").value="";
    document.querySelector("input[name='vpRA'][value='all']").checked=true;
    // Reset field list ke semua tercentang
    buildFieldCheckboxes("vpAddFieldList",_vpFormSchema,[]);
    _vpAddFilters={};
    renderFilterChips("vpAddFilterList",{},"removeVpAddFilter");
    await refreshVpPermList();
  }catch(e){alert("Gagal: "+e.message);}
}

async function removeViewerPerm(permId,viewerName){
  if(!confirm(`Cabut akses "${viewerName}" dari kuesioner ini?`))return;
  try{await api("/api/viewer-permissions/"+permId,{method:"DELETE"});await refreshVpPermList();}
  catch(e){alert("Gagal: "+e.message);}
}

// --- Dialog konfigurasi detail (field visibility + respondents) ---
let _vpdPermId=null, _vpdFormId=null;
async function openVpDetail(permId,viewerName,formId){
  _vpdPermId=permId; _vpdFormId=formId;
  document.getElementById("vpdViewerName").textContent=viewerName;

  try{
    // Load semua data yang dibutuhkan secara paralel
    const [curPerm,allowedData,formData,respondentsData]=await Promise.all([
      api("/api/viewer-permissions/"+permId),
      api("/api/viewer-permissions/"+permId+"/respondents").catch(()=>({respondents:[]})),
      api("/api/forms/"+formId),
      api("/api/forms/"+formId+"/respondents").catch(()=>({respondents:[]}))
    ]);
    _vpFormSchema=formData.schema;

    // Set radio akses responden
    document.querySelector(`input[name='vpdRA'][value='${curPerm.respondentAccess}']`).checked=true;
    toggleRespondentSection(curPerm.respondentAccess==="selected");

    // Isi daftar field
    buildVpdFieldList(formData.schema,curPerm.visibleFields||[]);

    // Isi filter variabel
    _vpdFilters=curPerm.fieldFilters||{};
    buildFieldOptions(formData.schema,"vpdFilterField");
    renderFilterChips("vpdFilterList",_vpdFilters,"removeVpdFilter");

    // Isi daftar allowed respondents
    renderAllowedRespondents(allowedData.respondents||[]);

    // Isi picker responden (hanya yang belum ditambahkan)
    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((allowedData.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (respondentsData.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");

    vpDetailDlg.showModal();
  }catch(e){alert("Gagal memuat: "+e.message);}
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
    // Reload section
    const [perm,formRespondents]=await Promise.all([
      api("/api/viewer-permissions/"+_vpdPermId+"/respondents"),
      api("/api/forms/"+_vpdFormId+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedRespondents(perm.respondents||[]);
    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){alert("Gagal: "+e.message);}
}

async function removeAllowedRespondent(id){
  try{
    await api("/api/viewer-respondents/"+id,{method:"DELETE"});
    const [perm,formRespondents]=await Promise.all([
      api("/api/viewer-permissions/"+_vpdPermId+"/respondents"),
      api("/api/forms/"+_vpdFormId+"/respondents").catch(()=>({respondents:[]}))
    ]);
    renderAllowedRespondents(perm.respondents||[]);
    const picker=document.getElementById("vpdRespondentPicker");
    const allowed=new Set((perm.respondents||[]).map(r=>r.respondentId));
    picker.innerHTML=`<option value="">— pilih responden —</option>`+
      (formRespondents.respondents||[]).filter(r=>!allowed.has(r.id)).map(r=>
        `<option value="${esc(r.id)}">${esc(r.name||r.email||r.id)}</option>`).join("");
  }catch(e){alert("Gagal: "+e.message);}
}

// buildFieldCheckboxes: render daftar checkbox variabel ke dalam elemen containerId.
// checked: array nama field yang dicentang; jika kosong ([]) semua dicentang (semua terlihat).
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

// Centang/kosongkan semua checkbox di panel tambah (vpAddFieldList)
function vpCheckAll(on){
  document.querySelectorAll("#vpAddFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
}

// (alias untuk backward compat, dipakai di openVpDetail)
function buildVpdFieldList(schema,checked){buildFieldCheckboxes("vpdFieldList",schema,checked);}

// Centang/kosongkan semua checkbox di panel edit detail (vpdFieldList)
function vpdCheckAll(on){
  document.querySelectorAll("#vpdFieldList input[type=checkbox]").forEach(cb=>{cb.checked=on;});
}

async function savePermDetail(){
  const respondentAccess=document.querySelector("input[name='vpdRA']:checked")?.value||"all";
  const checked=[...document.querySelectorAll("#vpdFieldList input:checked")].map(cb=>cb.value);
  // Jika semua field dicek = tidak perlu filter (kirim array kosong = semua terlihat)
  const total=document.querySelectorAll("#vpdFieldList input").length;
  const visibleFields=checked.length===total?[]:checked;
  try{
    await api("/api/viewer-permissions/"+_vpdPermId,{
      method:"PUT",body:JSON.stringify({respondentAccess,visibleFields,fieldFilters:_vpdFilters})
    });
    vpDetailDlg.close();
    await refreshVpPermList();
  }catch(e){alert("Gagal menyimpan: "+e.message);}
}

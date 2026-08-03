const token=localStorage.getItem("eform_token");
if(!token) location.replace("/login");
const H={"Authorization":"Bearer "+token,"Content-Type":"application/json"};
const $=s=>document.querySelector(s);
const esc=s=>String(s??"").replace(/[&<>"]/g,c=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"}[c]));

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
let MY_ID="";
let ACTIVE_NAV="forms";
let ACTIVE_USER_SUBTAB="viewer";
(async()=>{
  try{
    const me=await api("/api/auth/me");
    MY_ROLE=me.role||"admin";
    MY_ID=me.id||"";
    const uname=me.username||"",urole=me.role||"";
    const av=$("#userAvatar");if(av)av.textContent=uname.charAt(0).toUpperCase()||"?";
    const un=$("#userName");if(un)un.textContent=uname;
    const ur=$("#userRole");if(ur)ur.textContent=urole;
    const dn=$("#uddName");if(dn)dn.textContent=uname;
    const dr=$("#uddRole");if(dr)dr.textContent=urole;
    setupAdminMenu();
  }catch(e){}
  load();
})();

(function(){
  const userBtn=document.getElementById("userBtn");
  const dropdown=document.getElementById("userDropdown");
  if(userBtn&&dropdown){
    userBtn.addEventListener("click",e=>{e.stopPropagation();dropdown.hidden=!dropdown.hidden;});
    document.addEventListener("click",e=>{if(!dropdown.hidden&&!dropdown.contains(e.target))dropdown.hidden=true;});
  }
})();

function setupAdminMenu(){
  const navUsersBtn=$("#navUsersBtn");
  const btnNewForm=$("#btnNewForm");
  const canManageUsers=MY_ROLE==="superadmin"||MY_ROLE==="admin";
  if(navUsersBtn) navUsersBtn.hidden=!canManageUsers;
  if(btnNewForm) btnNewForm.style.display=canManageUsers?"":"none";

  // Sub-tab Admin: hanya superadmin
  const subtabAdminBtn=$("#subtabAdminBtn");
  if(subtabAdminBtn) subtabAdminBtn.hidden=MY_ROLE!=="superadmin";

  // Default sub-tab berdasar role
  ACTIVE_USER_SUBTAB=MY_ROLE==="superadmin"?"admin":"viewer";

  if(!canManageUsers){ switchNav("forms"); return; }

  $("#navFormsBtn")?.addEventListener("click",()=>switchNav("forms"));
  navUsersBtn?.addEventListener("click",()=>switchNav("users"));
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

function switchNav(nav){
  ACTIVE_NAV=nav;
  const formsNav=$("#navFormsBtn");
  const usersNav=$("#navUsersBtn");
  const formsSection=$("#formsSection");
  const usersSection=$("#usersSection");
  const newFormBtn=$("#btnNewForm");
  const isUsers=nav==="users";
  const canCreateForm=MY_ROLE==="admin"||MY_ROLE==="superadmin";
  formsSection.hidden=isUsers;
  usersSection.hidden=!isUsers;
  (isUsers?usersSection:formsSection)?.classList.add("fade-in");
  setTimeout(()=>(isUsers?usersSection:formsSection)?.classList.remove("fade-in"),200);
  formsNav?.classList.toggle("active",!isUsers);
  usersNav?.classList.toggle("active",isUsers);
  if(newFormBtn) newFormBtn.style.display=(!isUsers&&canCreateForm)?"":"none";
  if(isUsers){
    switchUserSubTab(ACTIVE_USER_SUBTAB);
  }
}

/* ======================================================
   DAFTAR KUESIONER — klik baris untuk membuka halaman pengelolaan
   ====================================================== */

async function load(){
  try{
    const {forms}=await api("/api/forms");
    const rows=$("#rows");
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    const colCount=canViewResults?4:3;
    if(!forms||!forms.length){rows.innerHTML=`<tr><td colspan="${colCount}" class="empty">Belum ada kuesioner. Klik “+ Kuesioner baru”.</td></tr>`;return;}
    const counts=canViewResults
      ? await Promise.all(forms.map(f=>api("/api/forms/"+f.id+"/responses?limit=1").then(d=>d.total).catch(()=>0)))
      : forms.map(()=>0);
    rows.innerHTML=forms.map((f,i)=>`<tr onclick="location.href='/manage?id=${f.id}'" style="cursor:pointer">
      <td><b>${esc(f.title)}</b><div class="muted">${esc(f.slug)}</div></td>
      <td><span class="tag ${f.status}">${f.status}</span></td>
      <td class="muted">${new Date(f.updatedAt).toLocaleString("id-ID")}</td>
      ${canViewResults?`<td>${counts[i]}</td>`:""}
    </tr>`).join("");
  }catch(e){
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    $("#rows").innerHTML=`<tr><td colspan="${canViewResults?4:3}" class="empty">${esc(e.message)}</td></tr>`;
  }
}

/* ======================================================
   MANAJEMEN USER — admin/superadmin
   ====================================================== */

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
  if(id===MY_ID){adminToast("Tidak bisa menghapus akun sendiri.",true);return;}
  adminConfirm(`Hapus user "${name}"? Tindakan ini tidak bisa dibatalkan.`,async()=>{
    try{
      await api("/api/users/"+id,{method:"DELETE"});
      await loadUsers();
    }catch(e){adminToast("Gagal: "+e.message,true);}
  });
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

$("#logout").addEventListener("click",()=>{localStorage.removeItem("eform_token");localStorage.removeItem("eform_user");location.replace("/login");});
$("#refresh").addEventListener("click",()=>{
  if(ACTIVE_NAV==="users"){
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
  }catch(e){adminToast("Gagal menyimpan: "+e.message,true);}
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
  adminConfirm(`Hapus viewer "${name}"? Semua akses kuesioner viewer ini akan ikut dihapus.`,async()=>{
    try{await api("/api/viewers/"+id,{method:"DELETE"});await loadViewersTab();}
    catch(e){adminToast("Gagal: "+e.message,true);}
  });
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
  }catch(e){adminToast("Gagal menyimpan: "+e.message,true);}
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
  adminConfirm(`Hapus editor "${name}"? Semua akses kuesioner editor ini akan ikut dihapus.`,async()=>{
    try{await api("/api/editors/"+id,{method:"DELETE"});await loadEditorsTab();}
    catch(e){adminToast("Gagal: "+e.message,true);}
  });
}

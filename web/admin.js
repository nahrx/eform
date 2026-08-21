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
  if(r.status===401){localStorage.removeItem("eform_token");location.replace("/login");throw new Error("session expired");}
  const ct=r.headers.get("content-type")||""; const data=ct.includes("json")?await r.json():null;
  if(!r.ok) throw new Error((data&&data.error)||("HTTP "+r.status));
  return data;
}

let MY_ROLE="admin";
let MY_ID="";
let ACTIVE_NAV="forms";
(async()=>{
  try{
    // Started in admin.html's head so it runs alongside the script downloads, and
    // shared with i18n.js so the page makes one /api/auth/me rather than two.
    const meRes=await window.__meOnce();
    if(meRes.status===401){localStorage.removeItem("eform_token");location.replace("/login");return;}
    if(!meRes.data)throw new Error("HTTP "+meRes.status);
    const me=meRes.data;
    MY_ROLE=me.role||"admin";
    MY_ID=me.id||"";
    const uname=me.username||"",urole=me.role||"";
    const av=$("#userAvatar");if(av)av.textContent=uname.charAt(0).toUpperCase()||"?";
    const un=$("#userName");if(un)un.textContent=uname;
    const ur=$("#userRole");if(ur)ur.textContent=urole;
    const dn=$("#uddName");if(dn)dn.textContent=uname;
    const dr=$("#uddRole");if(dr)dr.textContent=urole;
    setupAdminMenu();
  }catch(e){
    // A 401 has already redirected inside api(), so the shell stays hidden and the
    // stale dashboard is never shown. Anything else — the server down, no network —
    // must not leave a blank page, so fall through and reveal it.
  }
  revealAdminShell();
  load();
})();
/* Undoes the pre-paint gate in admin.html. Safe to call more than once. */
function revealAdminShell(){document.documentElement.classList.remove("auth-checking");}

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
  const navAuditBtn=$("#navAuditBtn");
  const btnNewForm=$("#btnNewForm");
  const canManageUsers=MY_ROLE==="superadmin";
  const canCreateForm=MY_ROLE==="admin"||MY_ROLE==="superadmin";
  if(navUsersBtn) navUsersBtn.hidden=!canManageUsers;
  if(navAuditBtn) navAuditBtn.hidden=!canManageUsers;
  if(btnNewForm) btnNewForm.style.display=canCreateForm?"":"none";

  if(!canCreateForm){ switchNav("forms"); return; }

  $("#navFormsBtn")?.addEventListener("click",()=>switchNav("forms"));
  if(canManageUsers){
    navUsersBtn?.addEventListener("click",()=>switchNav("users"));
    $("#refreshUsers")?.addEventListener("click",loadUsers);
    $("#btnCreateUser")?.addEventListener("click",createUserFromPanel);
    navAuditBtn?.addEventListener("click",()=>switchNav("audit"));
    $("#refreshAudit")?.addEventListener("click",()=>loadAudit(0));
    $("#auditPrev")?.addEventListener("click",()=>loadAudit(AUDIT_PAGE-1));
    $("#auditNext")?.addEventListener("click",()=>loadAudit(AUDIT_PAGE+1));
  }

  const wantedNav=new URLSearchParams(location.search).get("tab");
  const allowed=["users","audit"].includes(wantedNav)&&canManageUsers;
  switchNav(allowed?wantedNav:"forms");
}

function switchNav(nav){
  ACTIVE_NAV=nav;
  const canCreateForm=MY_ROLE==="admin"||MY_ROLE==="superadmin";
  const sections={forms:"#formsSection",users:"#usersSection",audit:"#auditSection"};
  const navs={forms:"#navFormsBtn",users:"#navUsersBtn",audit:"#navAuditBtn"};
  Object.entries(sections).forEach(([key,sel])=>{
    const el=$(sel);
    if(el) el.hidden=key!==nav;
    $(navs[key])?.classList.toggle("active",key===nav);
  });
  const shown=$(sections[nav]);
  if(shown){shown.classList.add("fade-in");setTimeout(()=>shown.classList.remove("fade-in"),200);}

  const newFormBtn=$("#btnNewForm");
  if(newFormBtn) newFormBtn.style.display=(nav==="forms"&&canCreateForm)?"":"none";

  if(nav==="users") loadUsers();
  else if(nav==="audit") loadAudit(0);

  const params=new URLSearchParams(location.search);
  if(nav==="forms") params.delete("tab"); else params.set("tab",nav);
  const qs=params.toString();
  history.replaceState(null,"",location.pathname+(qs?"?"+qs:""));
}

/* ======================================================
   RIWAYAT AKSI (audit) — superadmin
   ====================================================== */

let AUDIT_PAGE=0;
const AUDIT_SIZE=50;

async function loadAudit(page){
  if(page<0) return;
  const rows=$("#auditRows");
  if(!rows) return;
  rows.innerHTML='<tr><td colspan="6" class="empty">Loading…</td></tr>';
  try{
    const d=await api(`/api/activity-logs?limit=${AUDIT_SIZE}&offset=${page*AUDIT_SIZE}`);
    AUDIT_PAGE=page;
    const logs=d.logs||[];
    if(!logs.length){
      rows.innerHTML='<tr><td colspan="6" class="empty">No actions recorded yet.</td></tr>';
    }else{
      rows.innerHTML=logs.map(l=>`<tr>
        <td class="muted">${new Date(l.createdAt).toLocaleString("id-ID")}</td>
        <td><b>${esc(l.actorName||"—")}</b><div class="muted">${esc(l.actorRole||"")}</div></td>
        <td><span class="tag${/delete|revoke/.test(l.action)?" archived":""}">${esc(l.action)}</span></td>
        <td>${esc(l.targetLabel||l.targetId||"—")}</td>
        <td class="muted">${esc(l.ip||"—")}</td>
        <td class="muted">${esc(l.detail||"")}</td>
      </tr>`).join("");
    }
    const totalPages=Math.max(1,Math.ceil((d.total||0)/AUDIT_SIZE));
    $("#auditPageInfo").textContent=`Hal. ${page+1} / ${totalPages} · ${d.total||0} aksi`;
    $("#auditPrev").disabled=page<=0;
    $("#auditNext").disabled=page>=totalPages-1;
  }catch(e){
    rows.innerHTML=`<tr><td colspan="6" class="empty">${esc(e.message)}</td></tr>`;
  }
}

/* ======================================================
   FORM LIST — click a row to open its management page
   ====================================================== */

async function load(){
  try{
    const {forms}=await api("/api/forms");
    const rows=$("#rows");
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    const colCount=(canViewResults?4:3)+1;
    if(!forms||!forms.length){rows.innerHTML=`<tr><td colspan="${colCount}" class="empty">No forms yet. Click “+ New form”.</td></tr>`;return;}
    // The response count already ships with /api/forms — this used to make one request HTTP
    // per form just to fetch the number.
    rows.innerHTML=forms.map(f=>`<tr onclick="location.href='/manage?id=${f.id}'" style="cursor:pointer">
      <td><b>${esc(f.title)}</b><div class="muted">${esc(f.slug)}</div></td>
      <td><span class="tag ${f.status}">${f.status}</span></td>
      <td class="muted">${new Date(f.updatedAt).toLocaleString("id-ID")}</td>
      ${canViewResults?`<td>${f.responseCount||0}</td>`:""}
      <td style="text-align:center"><button class="row-menu-btn" type="button" onclick="event.stopPropagation();toggleRowMenu(event,'${f.id}')">⋮</button></td>
    </tr>`).join("");
  }catch(e){
    const canViewResults=MY_ROLE!=="editor";
    const answersTh=$("#thAnswers");
    if(answersTh) answersTh.style.display=canViewResults?"":"none";
    $("#rows").innerHTML=`<tr><td colspan="${(canViewResults?4:3)+1}" class="empty">${esc(e.message)}</td></tr>`;
  }
}

/* ======================================================
   FORM LIST — the per-row ⋮ menu (Open Builder / View Responses)
   ====================================================== */

let _rowMenuFormId=null;

function toggleRowMenu(e,formId){
  const menu=document.getElementById("rowMenu");
  if(!menu) return;
  if(!menu.hidden&&_rowMenuFormId===formId){menu.hidden=true;return;}
  _rowMenuFormId=formId;
  const r=e.currentTarget.getBoundingClientRect();
  menu.style.top=(r.bottom+4)+"px";
  menu.style.left=Math.max(8,r.right-170)+"px";
  menu.hidden=false;
}

document.addEventListener("click",e=>{
  const menu=document.getElementById("rowMenu");
  if(menu&&!menu.hidden&&!menu.contains(e.target)) menu.hidden=true;
});

function openBuilderFromMenu(){
  if(_rowMenuFormId) location.href="/builder?id="+_rowMenuFormId;
}
function openResponsesFromMenu(){
  if(_rowMenuFormId) location.href="/responses?id="+_rowMenuFormId;
}

/* ======================================================
   MANAJEMEN USER — admin/superadmin
   ====================================================== */

let _usersCache=[];

async function loadUsers(){
  if(MY_ROLE!=="superadmin") return;
  const rows=$("#userRows");
  if(!rows) return;
  rows.innerHTML='<tr><td colspan="6" class="empty">Loading…</td></tr>';
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
  if(!_usersCache.length){rows.innerHTML='<tr><td colspan="6" class="empty">No users yet.</td></tr>';return;}
  rows.innerHTML=_usersCache.map(u=>`<tr id="urow-${u.id}">
    <td><b>${esc(u.username||"-")}</b></td>
    <td class="muted">${esc(u.email||"-")}</td>
    <td><span class="tag">${esc(u.role||"-")}</span></td>
    <td><span class="tag ${u.isActive?"published":"archived"}">${u.isActive?"Active":"Inactive"}</span></td>
    <td class="muted">${u.createdAt?new Date(u.createdAt).toLocaleString("id-ID"):"-"}</td>
    <td style="text-align:right;white-space:nowrap">
      <button class="btn" style="font-size:12px;padding:3px 8px" onclick="editAdminUser('${u.id}')">Edit</button>
      <button class="btn danger" style="font-size:12px;padding:3px 8px" onclick="deleteAdminUser('${u.id}','${esc(u.username)}')"${u.id===MY_ID?' disabled title="Cannot delete your own account"':""}>Delete</button>
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
        <div style="font-size:11px;color:var(--muted);margin-bottom:3px">New password <span style="font-weight:normal">(leave blank if unchanged)</span></div>
        <input id="uepw-${id}" type="password" style="width:100%;font-size:13px;padding:6px 8px;border:1px solid var(--line);border-radius:6px" placeholder="min. 6 characters">
      </div>
      <div style="display:flex;gap:6px;flex-shrink:0">
        <button class="btn primary" style="font-size:13px" onclick="saveAdminUser('${id}')">Save</button>
        <button class="btn" style="font-size:13px" onclick="cancelAdminUserEdit('${id}')">Cancel</button>
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
  if(!username){if(msg)msg.textContent="Username is required.";return;}
  if(password&&password.length<6){if(msg)msg.textContent="Password must be at least 6 characters.";return;}
  if(msg)msg.textContent="";
  try{
    const body={username,email,role};
    if(password) body.password=password;
    await api("/api/users/"+id,{method:"PATCH",body:JSON.stringify(body)});
    const u=_usersCache.find(x=>x.id===id);
    if(u){u.username=username;u.email=email;u.role=role;}
    document.getElementById("uedit-"+id)?.remove();
    _renderUsersTab();
  }catch(e){if(msg)msg.textContent="Failed: "+e.message;}
}

async function deleteAdminUser(id,name){
  if(id===MY_ID){adminToast("Cannot delete your own account.",true);return;}
  adminConfirm(`Delete user "${name}"? This action cannot be undone.`,async()=>{
    try{
      await api("/api/users/"+id,{method:"DELETE"});
      await loadUsers();
    }catch(e){adminToast("Failed: "+e.message,true);}
  });
}

async function createUserFromPanel(){
  const username=(""+($("#uUsername")?.value||"")).trim();
  const email=(""+($("#uEmail")?.value||"")).trim();
  const password=(""+($("#uPassword")?.value||"")).trim();
  const role=(""+($("#uRole")?.value||"admin")).trim();
  const msg=$("#userMsg");

  if(!username){
    if(msg) msg.textContent="Username is required.";
    $("#uUsername")?.focus();
    return;
  }
  if(password.length<6){
    if(msg) msg.textContent="Password must be at least 6 characters.";
    $("#uPassword")?.focus();
    return;
  }

  const btn=$("#btnCreateUser");
  if(btn){btn.disabled=true;btn.textContent="Creating…";}
  if(msg) msg.textContent="";
  try{
    await api("/api/users",{
      method:"POST",
      body:JSON.stringify({username,email,password,role})
    });
    if(msg) msg.textContent="User created successfully.";
    if($("#uUsername")) $("#uUsername").value="";
    if($("#uEmail")) $("#uEmail").value="";
    if($("#uPassword")) $("#uPassword").value="";
    if($("#uRole")) $("#uRole").value="admin";
    await loadUsers();
  }catch(e){
    if(msg) msg.textContent="Failed: "+e.message;
  }finally{
    if(btn){btn.disabled=false;btn.textContent="+ Create User";}
  }
}

$("#logout").addEventListener("click",()=>{localStorage.removeItem("eform_token");localStorage.removeItem("eform_user");location.replace("/login");});
$("#refresh").addEventListener("click",()=>{
  if(ACTIVE_NAV==="users"){ loadUsers(); return; }
  if(ACTIVE_NAV==="audit"){ loadAudit(AUDIT_PAGE); return; }
  load();
});

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

(async()=>{
  try{const me=await api("/api/auth/me");$("#who").textContent=me.username+" · "+me.role;}catch(e){}
  load();
})();

async function load(){
  try{
    const {forms}=await api("/api/forms");
    const rows=$("#rows");
    if(!forms||!forms.length){rows.innerHTML='<tr><td colspan="5" class="empty">Belum ada kuesioner. Klik “+ Kuesioner baru”.</td></tr>';return;}
    const counts=await Promise.all(forms.map(f=>api("/api/forms/"+f.id+"/responses?limit=1").then(d=>d.total).catch(()=>0)));
    rows.innerHTML=forms.map((f,i)=>`<tr>
      <td><b>${esc(f.title)}</b><div class="muted">${esc(f.slug)}</div></td>
      <td><span class="tag ${f.status}">${f.status}</span></td>
      <td class="muted">${new Date(f.updatedAt).toLocaleString("id-ID")}</td>
      <td>${counts[i]} ${counts[i]?`· <a href="/api/forms/${f.id}/responses.csv" onclick="return dl(event,'${f.id}')">CSV</a>`:""}</td>
      <td><div class="acts">
        <button class="btn" onclick="location.href='/builder?id=${f.id}'">Buka</button>
        <button class="btn" onclick="togglePub('${f.id}','${f.status}')">${f.status==="published"?"Tarik":"Publikasikan"}</button>
        <button class="btn" onclick="openShare('${f.id}','${esc(f.title)}','${f.status}')">Bagikan</button>
        <button class="btn" onclick="location.href='/responses?id=${f.id}'">Jawaban${counts[i]>0?` (${counts[i]})`:""}</button>
        <button class="btn danger" onclick="del('${f.id}','${esc(f.title)}')" ${counts[i]>0?'disabled title="Tidak dapat dihapus karena sudah ada jawaban"':""}>Hapus</button>
      </div></td></tr>`).join("");
  }catch(e){ $("#rows").innerHTML=`<tr><td colspan="5" class="empty">${esc(e.message)}</td></tr>`; }
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
$("#refresh").addEventListener("click",load);

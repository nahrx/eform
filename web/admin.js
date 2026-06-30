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
        <button class="btn danger" onclick="del('${f.id}','${esc(f.title)}')">Hapus</button>
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
  $("#shareList").innerHTML="Memuat…"; shareDlg.showModal(); refreshShares();
}
async function refreshShares(){
  try{
    const {shares}=await api("/api/forms/"+shareFormId+"/shares");
    $("#shareList").innerHTML = (shares&&shares.length)? shares.map(s=>`
      <div class="share">
        <div><b>${esc(s.label||"(tanpa label)")}</b> ${s.isActive?"":"<span class='tag archived'>nonaktif</span>"} ${s.hasPassword?"🔒":""} · ${s.viewCount}×</div>
        <div style="margin:6px 0"><code>${esc(s.shareUrl)}</code></div>
        <div class="acts">
          <button class="btn" onclick="navigator.clipboard.writeText('${esc(s.shareUrl)}')">Salin</button>
          <a class="btn" href="${esc(s.shareUrl)}" target="_blank">Buka</a>
          ${s.isActive?`<button class="btn danger" onclick="revoke('${s.id}')">Cabut</button>`:""}
        </div>
      </div>`).join("") : "<div class='muted'>Belum ada tautan.</div>";
  }catch(e){ $("#shareList").innerHTML=esc(e.message); }
}
async function revoke(id){ try{await api("/api/shares/"+id,{method:"DELETE"});refreshShares();}catch(e){alert(e.message);} }
$("#makeShare").addEventListener("click",async()=>{
  try{
    await api("/api/forms/"+shareFormId+"/shares",{method:"POST",body:JSON.stringify({
      label:$("#shareLabel").value.trim(),
      allowResponses:$("#shareAllow").checked,
      password:$("#sharePw").value
    })});
    $("#shareLabel").value="";$("#sharePw").value="";
    refreshShares();
  }catch(e){alert(e.message);}
});

$("#logout").addEventListener("click",()=>{localStorage.removeItem("eform_token");localStorage.removeItem("eform_user");location.replace("/login");});
$("#refresh").addEventListener("click",load);

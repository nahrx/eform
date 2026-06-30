  const $=s=>document.querySelector(s);
  // sudah login? langsung ke admin
  if(localStorage.getItem("eform_token")) location.replace("/admin");
  async function login(){
    const btn=$("#btn"); const err=$("#err"); err.style.display="none";
    const username=$("#u").value.trim(), password=$("#p").value;
    if(!username||!password){err.textContent="Username dan password wajib diisi";err.style.display="block";return;}
    btn.disabled=true; btn.textContent="Memproses…";
    try{
      const r=await fetch("/api/auth/login",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({username,password})});
      const data=await r.json();
      if(!r.ok) throw new Error(data.error||"Gagal masuk");
      localStorage.setItem("eform_token",data.token);
      localStorage.setItem("eform_user",JSON.stringify(data.user));
      location.replace("/admin");
    }catch(e){ err.textContent=e.message; err.style.display="block"; }
    finally{ btn.disabled=false; btn.textContent="Masuk"; }
  }
  $("#btn").addEventListener("click",login);
  $("#p").addEventListener("keydown",e=>{if(e.key==="Enter")login();});

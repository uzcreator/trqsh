package api

// adminPageHTML is the self-contained approve.<base> console: a username/password
// gate and a full operations dashboard (fleet overview, tunnels, users, orgs) plus
// the subscription grant/revoke tools. No external resources (CSP-safe); every
// call is same-origin to /admin/* and /v1/admin/*.
const adminPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>trqsh · admin</title>
<style>
  :root{color-scheme:dark}
  *{box-sizing:border-box}
  body{margin:0;font:15px/1.5 system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:#060908;color:#e6f0ea}
  body.authed{display:block}
  body:not(.authed){display:flex;min-height:100vh;align-items:center;justify-content:center;padding:24px}
  a{color:#7fe3b1}
  .card{width:100%;max-width:420px;background:#0d1512;border:1px solid #1c2b25;border-radius:14px;padding:22px}
  h1{margin:0 0 2px;font-size:18px}
  h2{font-size:14px;margin:0 0 10px;color:#cfe0d8;font-weight:600}
  .sub{color:#7d8c85;font-size:13px;margin:0 0 18px}
  label{display:block;font-size:12px;color:#9fb0a8;margin:12px 0 5px}
  input,select{width:100%;padding:9px 11px;background:#060908;border:1px solid #22332c;border-radius:8px;color:#e6f0ea;font-size:14px}
  input:focus,select:focus{outline:none;border-color:#17ac6c}
  button{padding:9px 12px;border:0;border-radius:8px;background:#17ac6c;color:#04120b;font-weight:600;font-size:14px;cursor:pointer}
  button:hover{background:#149a60}
  button.ghost{background:transparent;border:1px solid #22332c;color:#cfe0d8;font-weight:500}
  button.ghost:hover{background:#12201b}
  button.danger{background:transparent;border:1px solid #5a2530;color:#f2a9b3}
  button.danger:hover{background:#1c1114}
  .row{display:flex;gap:10px}
  .row>*{flex:1}
  .msg{margin-top:14px;padding:10px 12px;border-radius:8px;font-size:13px;display:none}
  .msg.err{display:block;background:#1c1114;border:1px solid #5a2530;color:#f2a9b3}
  .msg.ok{display:block;background:#0c1a13;border:1px solid #1f6e49;color:#7fe3b1}
  .info{margin-top:14px;padding:12px;border:1px solid #22332c;border-radius:8px;font-size:13px;display:none}
  .info b{color:#fff}
  .hidden{display:none}
  code{color:#7fe3b1}

  /* Dashboard shell */
  .wrap{max-width:1180px;margin:0 auto;padding:18px 20px 60px}
  .bar{display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid #16241d;padding-bottom:14px;margin-bottom:18px}
  .bar h1{font-size:16px}
  .brand{color:#17ac6c}
  .signout{font-size:13px;color:#9fb0a8;background:none;border:1px solid #22332c;padding:6px 12px}
  .signout:hover{color:#e6f0ea;background:#12201b}
  .tabs{display:flex;gap:6px;flex-wrap:wrap;margin-bottom:20px}
  .tabs button{background:transparent;border:1px solid #1c2b25;color:#9fb0a8;font-weight:500}
  .tabs button.active{background:#12201b;border-color:#17ac6c;color:#e6f0ea}
  .panel{display:none}
  .panel.active{display:block}
  .cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(165px,1fr));gap:12px;margin-bottom:22px}
  .stat{background:#0d1512;border:1px solid #1c2b25;border-radius:12px;padding:14px}
  .stat .k{font-size:12px;color:#7d8c85;margin-bottom:6px}
  .stat .v{font-size:22px;font-weight:700;color:#fff;letter-spacing:-.02em}
  .stat .d{font-size:12px;color:#17ac6c;margin-top:3px}
  .grid2{display:grid;grid-template-columns:1fr 1fr;gap:16px}
  @media(max-width:760px){.grid2{grid-template-columns:1fr}}
  .box{background:#0d1512;border:1px solid #1c2b25;border-radius:12px;padding:16px;margin-bottom:16px}
  .barrow{display:flex;align-items:center;gap:10px;margin:7px 0;font-size:13px}
  .barrow .lab{width:120px;color:#cfe0d8;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
  .barrow .track{flex:1;height:8px;background:#0a1410;border-radius:6px;overflow:hidden}
  .barrow .fill{height:100%;background:#17ac6c}
  .barrow .n{width:52px;text-align:right;color:#9fb0a8}
  .tablewrap{overflow-x:auto;border:1px solid #1c2b25;border-radius:12px}
  table{width:100%;border-collapse:collapse;font-size:13px}
  th,td{padding:9px 12px;border-bottom:1px solid #16241d;text-align:left;white-space:nowrap}
  th{color:#9fb0a8;font-weight:500;background:#0b120f;position:sticky;top:0}
  tbody tr:hover{background:#0b1310}
  td.mono,code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
  .pill{display:inline-block;padding:1px 8px;border-radius:20px;font-size:11px;border:1px solid #22332c;color:#cfe0d8}
  .pill.pro{border-color:#1f6e49;color:#7fe3b1}
  .pill.team{border-color:#2b5e8f;color:#8fc7f2}
  .pill.active{border-color:#1f6e49;color:#7fe3b1}
  .pill.closed{border-color:#3a4a43;color:#8b9a92}
  .toolbar{display:flex;gap:10px;align-items:center;margin-bottom:12px;flex-wrap:wrap}
  .toolbar input,.toolbar select{width:auto;min-width:180px}
  .toolbar button{padding:8px 12px}
  .pager{display:flex;gap:10px;align-items:center;justify-content:flex-end;margin-top:12px;color:#7d8c85;font-size:13px}
  .empty{padding:26px;text-align:center;color:#7d8c85}
  .muted{color:#7d8c85}
</style>
</head>
<body>
  <!-- Login -->
  <div id="login" class="card">
    <h1>trqsh admin</h1>
    <p class="sub">Sign in to manage the deployment.</p>
    <label for="u">Username</label>
    <input id="u" autocomplete="username" autofocus>
    <label for="p">Password</label>
    <input id="p" type="password" autocomplete="current-password">
    <div id="loginMsg" class="msg"></div>
    <button id="loginBtn" style="width:100%;margin-top:16px">Sign in</button>
  </div>

  <!-- Dashboard -->
  <div id="dash" class="wrap hidden">
    <div class="bar">
      <h1><span class="brand">trqsh</span> admin</h1>
      <button class="signout" id="signout">Sign out</button>
    </div>
    <div class="tabs">
      <button data-tab="overview" class="active">Overview</button>
      <button data-tab="tunnels">Tunnels</button>
      <button data-tab="users">Users</button>
      <button data-tab="orgs">Orgs</button>
      <button data-tab="grants">Grants</button>
    </div>

    <!-- Overview -->
    <div class="panel active" id="p-overview">
      <div class="toolbar"><button class="ghost" id="refreshOv">Refresh</button><span class="muted" id="ovTime"></span></div>
      <div class="cards" id="ovCards"></div>
      <div class="grid2">
        <div class="box"><h2>Orgs by plan</h2><div id="ovPlans"></div></div>
        <div class="box"><h2>Tunnels by country</h2><div id="ovGeo"></div></div>
      </div>
      <div class="box"><h2>Edge regions</h2><div id="ovRegions" class="muted"></div></div>
    </div>

    <!-- Tunnels -->
    <div class="panel" id="p-tunnels">
      <div class="toolbar">
        <label style="margin:0;display:flex;align-items:center;gap:6px;color:#cfe0d8"><input type="checkbox" id="tActive" style="width:auto"> Active only</label>
        <button class="ghost" id="tReload">Reload</button>
        <span class="muted" id="tTotal"></span>
      </div>
      <div class="tablewrap"><table><thead><tr>
        <th>Public URL</th><th>Type</th><th>Region</th><th>Location</th><th>Org</th><th>Bytes (in/out)</th><th>Started</th><th>Status</th>
      </tr></thead><tbody id="tBody"></tbody></table></div>
      <div class="pager"><button class="ghost" id="tPrev">Prev</button><span id="tPage"></span><button class="ghost" id="tNext">Next</button></div>
    </div>

    <!-- Users -->
    <div class="panel" id="p-users">
      <div class="toolbar">
        <input id="uSearch" placeholder="Search email or name…">
        <button class="ghost" id="uReload">Search</button>
      </div>
      <div class="tablewrap"><table><thead><tr>
        <th>Email</th><th>Name</th><th>Plan</th><th>Provider</th><th>Org</th><th>Joined</th>
      </tr></thead><tbody id="uBody"></tbody></table></div>
      <div class="pager"><button class="ghost" id="uPrev">Prev</button><span id="uPage"></span><button class="ghost" id="uNext">Next</button></div>
    </div>

    <!-- Orgs -->
    <div class="panel" id="p-orgs">
      <div class="toolbar">
        <select id="oPlan"><option value="">All plans</option><option value="free">Free</option><option value="pro">Pro</option><option value="team">Team</option></select>
        <button class="ghost" id="oReload">Filter</button>
      </div>
      <div class="tablewrap"><table><thead><tr>
        <th>Name</th><th>Org ID</th><th>Plan</th><th>Members</th><th>Expires</th><th>Created</th>
      </tr></thead><tbody id="oBody"></tbody></table></div>
      <div class="pager"><button class="ghost" id="oPrev">Prev</button><span id="oPage"></span><button class="ghost" id="oNext">Next</button></div>
    </div>

    <!-- Grants -->
    <div class="panel" id="p-grants">
      <div class="box" style="max-width:460px">
        <h2>Grant a subscription</h2>
        <p class="sub" style="margin-bottom:8px">Look up an account by email, then grant or revoke.</p>
        <label for="email">Account email</label>
        <div class="row">
          <input id="email" type="email" placeholder="user@example.com">
          <button class="ghost" id="lookupBtn" style="max-width:110px">Look up</button>
        </div>
        <div id="info" class="info"></div>
        <div class="row">
          <div><label for="plan">Plan</label><select id="plan"><option value="pro">Pro</option><option value="team">Team</option><option value="free">Free</option></select></div>
          <div><label for="months">Months</label><input id="months" type="number" min="1" max="60" value="1"></div>
        </div>
        <div class="row" style="margin-top:16px">
          <button id="grantBtn">Grant</button>
          <button class="danger" id="revokeBtn">Revoke to Free</button>
        </div>
        <div id="msg" class="msg"></div>
      </div>
    </div>
  </div>

<script>
  var $ = function(id){return document.getElementById(id)};
  function api(method, path, body){
    return fetch(path, {
      method: method,
      headers: body ? {'Content-Type':'application/json'} : {},
      body: body ? JSON.stringify(body) : undefined,
      credentials: 'same-origin'
    }).then(function(r){
      return r.text().then(function(t){
        var j = {}; try{ j = t ? JSON.parse(t) : {} }catch(e){}
        if(!r.ok) throw new Error(j.error || ('HTTP ' + r.status));
        return j;
      });
    });
  }
  function show(el){ el.classList.remove('hidden') }
  function hide(el){ el.classList.add('hidden') }
  function setMsg(el, text, ok){ el.textContent = text; el.className = 'msg ' + (ok ? 'ok' : 'err') }
  function esc(s){ return String(s==null?'':s).replace(/[&<>"]/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]}) }
  function fmtNum(n){ return (+n||0).toLocaleString() }
  function fmtBytes(n){ n=+n||0; var u=['B','KB','MB','GB','TB','PB']; var i=0; while(n>=1024&&i<u.length-1){n/=1024;i++} return (i?n.toFixed(1):n)+' '+u[i] }
  function fmtDate(v){ if(!v)return '—'; var d=new Date(v); if(isNaN(d.getTime()))return '—'; return d.toLocaleString() }
  function flag(cc){ if(!cc||cc.length!==2||cc==='??')return '🏳️'; cc=cc.toUpperCase(); return String.fromCodePoint(0x1F1E6+cc.charCodeAt(0)-65,0x1F1E6+cc.charCodeAt(1)-65) }
  function planPill(p){ return '<span class="pill '+esc(p)+'">'+esc(p)+'</span>' }

  var LIMIT = 50;
  var st = { tunnels:{off:0}, users:{off:0,q:''}, orgs:{off:0,plan:''} };

  function enterDash(){
    document.body.classList.add('authed');
    hide($('login')); show($('dash'));
    loadOverview();
  }
  function leaveDash(){
    document.body.classList.remove('authed');
    show($('login')); hide($('dash'));
  }

  // Tabs.
  var tabButtons = document.querySelectorAll('.tabs button');
  for(var i=0;i<tabButtons.length;i++){
    tabButtons[i].onclick = function(){
      var name = this.getAttribute('data-tab');
      for(var j=0;j<tabButtons.length;j++) tabButtons[j].classList.remove('active');
      this.classList.add('active');
      var panels = document.querySelectorAll('.panel');
      for(var k=0;k<panels.length;k++) panels[k].classList.remove('active');
      $('p-'+name).classList.add('active');
      if(name==='overview') loadOverview();
      else if(name==='tunnels'){ st.tunnels.off=0; loadTunnels() }
      else if(name==='users'){ st.users.off=0; loadUsers() }
      else if(name==='orgs'){ st.orgs.off=0; loadOrgs() }
    };
  }

  // --- Overview ---
  function statCard(k,v,d){ return '<div class="stat"><div class="k">'+k+'</div><div class="v">'+v+'</div>'+(d?'<div class="d">'+d+'</div>':'')+'</div>' }
  function bars(obj, order){
    var keys = order || Object.keys(obj).sort(function(a,b){return obj[b]-obj[a]});
    var max = 1; for(var i=0;i<keys.length;i++) if(obj[keys[i]]>max) max=obj[keys[i]];
    if(!keys.length) return '<div class="muted">No data yet.</div>';
    var h='';
    for(var j=0;j<keys.length;j++){
      var k=keys[j], v=obj[k]||0, pct=Math.round(v/max*100);
      h+='<div class="barrow"><div class="lab">'+esc(k)+'</div><div class="track"><div class="fill" style="width:'+pct+'%"></div></div><div class="n">'+fmtNum(v)+'</div></div>';
    }
    return h;
  }
  function geoBars(obj){
    var keys = Object.keys(obj).sort(function(a,b){return obj[b]-obj[a]}).slice(0,10);
    var max=1; for(var i=0;i<keys.length;i++) if(obj[keys[i]]>max) max=obj[keys[i]];
    if(!keys.length) return '<div class="muted">No tunnel sessions yet.</div>';
    var h='';
    for(var j=0;j<keys.length;j++){
      var k=keys[j],v=obj[k],pct=Math.round(v/max*100);
      h+='<div class="barrow"><div class="lab">'+flag(k)+' '+esc(k)+'</div><div class="track"><div class="fill" style="width:'+pct+'%"></div></div><div class="n">'+fmtNum(v)+'</div></div>';
    }
    return h;
  }
  function loadOverview(){
    api('GET','/v1/admin/stats').then(function(d){
      var s=d.stats||{};
      $('ovCards').innerHTML =
        statCard('Users', fmtNum(s.users), '+'+fmtNum(s.new_users_7d)+' this week') +
        statCard('Orgs', fmtNum(s.orgs)) +
        statCard('Active tunnels', fmtNum(s.active_tunnels)) +
        statCard('Total tunnels', fmtNum(s.total_tunnels)) +
        statCard('API keys', fmtNum(s.api_keys)) +
        statCard('Custom domains', fmtNum(s.custom_domains)) +
        statCard('Subdomains', fmtNum(s.reserved_subdomains)) +
        statCard('Traffic 30d', fmtBytes((+s.bytes_in_30d||0)+(+s.bytes_out_30d||0)), fmtNum(s.requests_30d)+' reqs');
      $('ovPlans').innerHTML = bars(s.orgs_by_plan||{}, ['free','pro','team']);
      $('ovGeo').innerHTML = geoBars(d.geo||{});
      var regs=(d.regions||[]).map(function(r){return flag(r.country)+' <code>'+esc(r.code)+'</code> '+esc(r.name)+' — <span class="muted">'+esc(r.endpoint)+'</span>'});
      $('ovRegions').innerHTML = regs.length? regs.join('<br>') : 'No regions configured.';
      $('ovTime').textContent = 'as of '+fmtDate(d.generated_at);
    }).catch(function(e){ $('ovCards').innerHTML='<div class="empty">'+esc(e.message)+'</div>' });
  }
  $('refreshOv').onclick = loadOverview;

  // --- Tunnels ---
  function loadTunnels(){
    var q='?active='+($('tActive').checked?'true':'false')+'&limit='+LIMIT+'&offset='+st.tunnels.off;
    api('GET','/v1/admin/tunnels'+q).then(function(d){
      var rows=(d.sessions||[]);
      var b='';
      for(var i=0;i<rows.length;i++){ var t=rows[i];
        var loc=(t.country?flag(t.country)+' '+esc(t.country):'')+(t.city?' '+esc(t.city):'');
        b+='<tr><td class="mono">'+esc(t.public_url||'—')+'</td><td>'+esc(t.type)+'</td><td>'+esc(t.region||'—')+'</td><td>'+(loc||'<span class="muted">—</span>')+
           '</td><td class="mono muted">'+esc(t.org_id||'')+'</td><td>'+fmtBytes(t.bytes_in)+' / '+fmtBytes(t.bytes_out)+
           '</td><td class="muted">'+fmtDate(t.started_at)+'</td><td><span class="pill '+esc(t.status)+'">'+esc(t.status)+'</span></td></tr>';
      }
      $('tBody').innerHTML = b || '<tr><td colspan="8" class="empty">No tunnels.</td></tr>';
      $('tTotal').textContent = fmtNum(d.total)+' total';
      pager('t', st.tunnels.off, rows.length, d.total);
    });
  }
  $('tReload').onclick=function(){ st.tunnels.off=0; loadTunnels() };
  $('tActive').onchange=function(){ st.tunnels.off=0; loadTunnels() };
  $('tPrev').onclick=function(){ if(st.tunnels.off>0){st.tunnels.off-=LIMIT; loadTunnels()} };
  $('tNext').onclick=function(){ st.tunnels.off+=LIMIT; loadTunnels() };

  // --- Users ---
  function loadUsers(){
    var q='?limit='+LIMIT+'&offset='+st.users.off+'&q='+encodeURIComponent(st.users.q);
    api('GET','/v1/admin/users'+q).then(function(d){
      var rows=(d.users||[]); var b='';
      for(var i=0;i<rows.length;i++){ var u=rows[i];
        b+='<tr><td class="mono">'+esc(u.email)+'</td><td>'+esc(u.name||'—')+'</td><td>'+(u.plan?planPill(u.plan):'<span class="muted">—</span>')+
           '</td><td class="muted">'+esc(u.oauth_provider||'password')+'</td><td class="mono muted">'+esc(u.org_id||'')+'</td><td class="muted">'+fmtDate(u.created_at)+'</td></tr>';
      }
      $('uBody').innerHTML = b || '<tr><td colspan="6" class="empty">No users.</td></tr>';
      pager('u', st.users.off, rows.length, null);
    });
  }
  $('uReload').onclick=function(){ st.users.q=$('uSearch').value.trim(); st.users.off=0; loadUsers() };
  $('uSearch').addEventListener('keydown',function(e){ if(e.key==='Enter') $('uReload').click() });
  $('uPrev').onclick=function(){ if(st.users.off>0){st.users.off-=LIMIT; loadUsers()} };
  $('uNext').onclick=function(){ st.users.off+=LIMIT; loadUsers() };

  // --- Orgs ---
  function loadOrgs(){
    var q='?limit='+LIMIT+'&offset='+st.orgs.off+'&plan='+encodeURIComponent(st.orgs.plan);
    api('GET','/v1/admin/orgs'+q).then(function(d){
      var rows=(d.orgs||[]); var b='';
      for(var i=0;i<rows.length;i++){ var o=rows[i];
        b+='<tr><td>'+esc(o.name||'—')+'</td><td class="mono muted">'+esc(o.id)+'</td><td>'+planPill(o.plan)+
           '</td><td>'+fmtNum(o.members)+'</td><td class="muted">'+fmtDate(o.plan_expires_at)+'</td><td class="muted">'+fmtDate(o.created_at)+'</td></tr>';
      }
      $('oBody').innerHTML = b || '<tr><td colspan="6" class="empty">No orgs.</td></tr>';
      pager('o', st.orgs.off, rows.length, null);
    });
  }
  $('oReload').onclick=function(){ st.orgs.plan=$('oPlan').value; st.orgs.off=0; loadOrgs() };
  $('oPrev').onclick=function(){ if(st.orgs.off>0){st.orgs.off-=LIMIT; loadOrgs()} };
  $('oNext').onclick=function(){ st.orgs.off+=LIMIT; loadOrgs() };

  function pager(p, off, got, total){
    var from = got? off+1 : 0;
    $(p+'Page').textContent = from+'–'+(off+got)+(total!=null? ' of '+fmtNum(total):'');
    $(p+'Prev').disabled = off<=0;
    $(p+'Next').disabled = got<LIMIT;
  }

  // --- Login / grants ---
  $('loginBtn').onclick = function(){
    $('loginMsg').className='msg';
    api('POST','/admin/login',{username:$('u').value,password:$('p').value})
      .then(enterDash).catch(function(e){ setMsg($('loginMsg'), e.message, false) });
  };
  $('p').addEventListener('keydown',function(e){ if(e.key==='Enter') $('loginBtn').click() });
  $('signout').onclick=function(){ api('POST','/admin/logout').finally(leaveDash) };

  function fmtExpiry(v){ if(!v) return 'no expiry'; var d=new Date(v); return d.toLocaleDateString() }
  function renderInfo(d){
    var el=$('info'); el.style.display='block';
    el.innerHTML='<b>'+esc(d.name||d.email)+'</b> &lt;'+esc(d.email)+'&gt;<br>Current plan: <code>'+esc(d.plan)+'</code> · '+fmtExpiry(d.plan_expires_at);
  }
  $('lookupBtn').onclick=function(){
    var email=$('email').value.trim(); if(!email) return;
    api('GET','/v1/admin/lookup?email='+encodeURIComponent(email)).then(renderInfo)
      .catch(function(e){ $('info').style.display='none'; setMsg($('msg'), e.message, false) });
  };
  $('grantBtn').onclick=function(){
    api('POST','/v1/admin/grant',{email:$('email').value.trim(),plan:$('plan').value,months:parseInt($('months').value,10)})
      .then(function(d){ renderInfo(d); setMsg($('msg'),'Granted '+d.plan+' until '+fmtExpiry(d.plan_expires_at), true) })
      .catch(function(e){ setMsg($('msg'), e.message, false) });
  };
  $('revokeBtn').onclick=function(){
    api('POST','/v1/admin/revoke',{email:$('email').value.trim()})
      .then(function(d){ renderInfo(d); setMsg($('msg'),'Reverted to Free.', true) })
      .catch(function(e){ setMsg($('msg'), e.message, false) });
  };

  // Resume an existing session.
  api('GET','/v1/admin/session').then(enterDash).catch(function(){});
</script>
</body>
</html>`

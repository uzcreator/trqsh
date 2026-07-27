package api

// adminPageHTML is the self-contained approve.<base> console: a username/password
// gate and a form to grant/revoke subscriptions by email. No external resources
// (CSP-safe); every call is same-origin to /admin/* and /v1/admin/*.
const adminPageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>trqsh · approvals</title>
<style>
  :root{color-scheme:dark}
  *{box-sizing:border-box}
  body{margin:0;font:15px/1.5 system-ui,-apple-system,Segoe UI,Roboto,sans-serif;
    background:#060908;color:#e6f0ea;display:flex;min-height:100vh;align-items:center;justify-content:center;padding:24px}
  .card{width:100%;max-width:420px;background:#0d1512;border:1px solid #1c2b25;border-radius:14px;padding:22px}
  h1{margin:0 0 2px;font-size:18px}
  .sub{color:#7d8c85;font-size:13px;margin:0 0 18px}
  label{display:block;font-size:12px;color:#9fb0a8;margin:12px 0 5px}
  input,select{width:100%;padding:9px 11px;background:#060908;border:1px solid #22332c;border-radius:8px;color:#e6f0ea;font-size:14px}
  input:focus,select:focus{outline:none;border-color:#17ac6c}
  button{width:100%;margin-top:16px;padding:10px;border:0;border-radius:8px;background:#17ac6c;color:#04120b;font-weight:600;font-size:14px;cursor:pointer}
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
  .top{display:flex;justify-content:space-between;align-items:center;margin-bottom:6px}
  .signout{font-size:12px;color:#7d8c85;background:none;border:0;width:auto;margin:0;padding:0;cursor:pointer}
  .signout:hover{color:#e6f0ea}
  code{color:#7fe3b1}
</style>
</head>
<body>
  <div class="card">
    <!-- Login -->
    <div id="login">
      <h1>trqsh approvals</h1>
      <p class="sub">Sign in to grant subscriptions.</p>
      <label for="u">Username</label>
      <input id="u" autocomplete="username" autofocus>
      <label for="p">Password</label>
      <input id="p" type="password" autocomplete="current-password">
      <div id="loginMsg" class="msg"></div>
      <button id="loginBtn">Sign in</button>
    </div>

    <!-- Console -->
    <div id="console" class="hidden">
      <div class="top">
        <h1>Grant a subscription</h1>
        <button class="signout" id="signout">Sign out</button>
      </div>
      <p class="sub">Enter the customer's account email, look them up, then grant.</p>

      <label for="email">Account email</label>
      <div class="row">
        <input id="email" type="email" placeholder="user@example.com">
        <button class="ghost" id="lookupBtn" style="max-width:110px">Look up</button>
      </div>

      <div id="info" class="info"></div>

      <div class="row">
        <div>
          <label for="plan">Plan</label>
          <select id="plan">
            <option value="pro">Pro</option>
            <option value="team">Team</option>
            <option value="free">Free</option>
          </select>
        </div>
        <div>
          <label for="months">Months</label>
          <input id="months" type="number" min="1" max="60" value="1">
        </div>
      </div>

      <button id="grantBtn">Grant subscription</button>
      <button class="danger" id="revokeBtn">Revoke to Free</button>
      <div id="msg" class="msg"></div>
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

  function enterConsole(){ hide($('login')); show($('console')); $('email').focus() }

  function fmtExpiry(v){ if(!v) return 'no expiry'; var d = new Date(v); return d.toLocaleDateString() }

  function renderInfo(d){
    var i = $('info');
    i.style.display = 'block';
    i.innerHTML = '<b>' + (d.name || d.email) + '</b> &lt;' + d.email + '&gt;<br>' +
      'Current plan: <code>' + d.plan + '</code> · ' + fmtExpiry(d.plan_expires_at);
  }

  // Login.
  $('loginBtn').onclick = function(){
    setMsg($('loginMsg'), '', true); $('loginMsg').className = 'msg';
    api('POST', '/admin/login', {username: $('u').value, password: $('p').value})
      .then(enterConsole)
      .catch(function(e){ setMsg($('loginMsg'), e.message, false) });
  };
  $('p').addEventListener('keydown', function(e){ if(e.key === 'Enter') $('loginBtn').click() });

  $('signout').onclick = function(){
    api('POST', '/admin/logout').finally(function(){ show($('login')); hide($('console')) });
  };

  // Lookup.
  $('lookupBtn').onclick = function(){
    var email = $('email').value.trim();
    if(!email) return;
    api('GET', '/v1/admin/lookup?email=' + encodeURIComponent(email))
      .then(renderInfo)
      .catch(function(e){ $('info').style.display='none'; setMsg($('msg'), e.message, false) });
  };

  // Grant.
  $('grantBtn').onclick = function(){
    var body = {email: $('email').value.trim(), plan: $('plan').value, months: parseInt($('months').value, 10)};
    api('POST', '/v1/admin/grant', body)
      .then(function(d){ renderInfo(d); setMsg($('msg'), 'Granted ' + d.plan + ' until ' + fmtExpiry(d.plan_expires_at), true) })
      .catch(function(e){ setMsg($('msg'), e.message, false) });
  };

  // Revoke.
  $('revokeBtn').onclick = function(){
    api('POST', '/v1/admin/revoke', {email: $('email').value.trim()})
      .then(function(d){ renderInfo(d); setMsg($('msg'), 'Reverted to Free.', true) })
      .catch(function(e){ setMsg($('msg'), e.message, false) });
  };

  // Resume an existing session.
  api('GET', '/v1/admin/session').then(enterConsole).catch(function(){});
</script>
</body>
</html>`

package quickconnect

const indexHTML = \`<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>اتصال سریع Chimera</title>
<style>
:root{color-scheme:dark;font-family:system-ui,-apple-system,Segoe UI,sans-serif;background:#0b1020;color:#edf2ff}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:20px}
main{width:min(100%,520px);background:#141c30;border:1px solid #2a3857;border-radius:20px;padding:26px;box-shadow:0 18px 60px #0005}
h1{font-size:1.65rem;margin:0 0 8px}.sub{color:#aab8d4;margin:0 0 24px;line-height:1.5}
label{display:block;font-weight:650;margin:18px 0 8px}input{width:100%;padding:13px 14px;border-radius:10px;border:1px solid #405174;background:#0c1426;color:#fff;font:inherit}
.hint{font-size:.85rem;color:#aab8d4;margin-top:7px;line-height:1.45}.actions{display:flex;gap:10px;margin-top:22px}
button{flex:1;padding:12px;border:0;border-radius:10px;font:inherit;font-weight:750;cursor:pointer}
#connect{background:#8ab4ff;color:#08152e}#disconnect{background:#293651;color:#edf2ff}button:disabled{opacity:.45;cursor:not-allowed}
#status{margin-top:18px;padding:12px;border-radius:10px;background:#0c1426;color:#c6d4ef;white-space:pre-wrap;overflow-wrap:anywhere;min-height:48px}
small{display:block;color:#92a3c4;margin-top:18px;line-height:1.5}
</style>
</head>
<body>
<main>
<h1>اتصال سریع Chimera</h1>
<p class="sub">برای اتصال فقط SNI و آدرس‌های لبه را وارد کن؛ نیازی به ساخت دستی فایل کانفیگ نیست.</p>
<form id="form">
<label for="sni">SNI / نام میزبان سرور</label>
<input id="sni" name="sni" autocomplete="url" placeholder="tunnel.example.com" required maxlength="253">
<div class="hint">نام میزبانی که برای اتصال امن سرور یا مسیر تونل تنظیم شده است.</div>
<label for="ips">IPهای لبهٔ CDN</label>
<input id="ips" name="ips" placeholder="IP اول، IP دوم" required maxlength="4096">
<div class="hint">IPها را با ویرگول یا فاصله جدا کن. فقط IP معتبر پذیرفته می‌شود.</div>
<div class="actions">
<button id="connect" type="submit">اتصال</button>
<button id="disconnect" type="button">قطع اتصال</button>
</div>
</form>
<div id="status" role="status" aria-live="polite">اتصال برقرار نیست. برای شروع، SNI و IPهای لبه را وارد کن.</div>
<small>نسخهٔ فعلی از موتور داخلی Chimera و پراکسی محلی SOCKS5 روی 127.0.0.1:1080 استفاده می‌کند؛ هنوز موتور Xray یا تونل کل‌دستگاه را پیاده‌سازی نمی‌کند. کلید و سرور سازگار باید از قبل تنظیم شده باشند. CDN یک گزینهٔ مسیر است و به‌خودی‌خود جایگزین تنظیمات سرور و احراز هویت نیست.</small>
</main>
<script>
const form=document.getElementById('form'),statusBox=document.getElementById('status'),connectBtn=document.getElementById('connect'),disconnectBtn=document.getElementById('disconnect');
async function api(path,body){const r=await fetch(path,{method:body?'POST':'GET',headers:body?{'Content-Type':'application/json'}:{},body:body?JSON.stringify(body):undefined,cache:'no-store'});let data={};try{data=await r.json()}catch{}if(!r.ok)throw new Error(data.error||data.message||'Request failed');return data}
function show(s){statusBox.textContent=s}
function busy(v){connectBtn.disabled=v;disconnectBtn.disabled=v}
form.addEventListener('submit',async e=>{e.preventDefault();busy(true);show('در حال بررسی مسیر و راه‌اندازی تونل…');try{const d=await api('/api/connect',{sni:document.getElementById('sni').value.trim(),edge_ips:document.getElementById('ips').value.trim()});show('اتصال برقرار شد.\nپراکسی SOCKS5: '+d.socks)}catch(e){show('اتصال برقرار نشد. SNI، IPها، کلید سرور و مسیر CDN را بررسی کن.\nجزئیات: '+e.message)}finally{busy(false)}});
disconnectBtn.addEventListener('click',async()=>{busy(true);try{await api('/api/disconnect',{});show('اتصال قطع شد.')}catch(e){show('قطع اتصال ناموفق بود: '+e.message)}finally{busy(false)}});
api('/api/status').then(d=>{if(d.connected)show('اتصال برقرار است.\nSOCKS5: '+d.socks)}).catch(()=>show('رابط محلی در دسترس نیست.'));
</script>
</body>
</html>\`

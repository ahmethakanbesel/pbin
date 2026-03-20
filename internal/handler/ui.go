package handler

import (
	"fmt"
	"net/http"
)

// UIHandler renders static HTML form pages for the web UI.
// It has no service dependencies — all form submissions use JS fetch to reach the API.
type UIHandler struct{}

// NewUIHandler constructs a UIHandler.
func NewUIHandler() *UIHandler {
	return &UIHandler{}
}

const (
	picoCSS = "https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.classless.min.css"

	// uiCSP is the Content-Security-Policy used on all three UI form pages.
	// connect-src 'self' is required so the inline JS fetch calls to /api/* work.
	uiCSP = "default-src 'none'; script-src 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'unsafe-inline' https://cdn.jsdelivr.net; form-action 'self'; connect-src 'self'"

	expiryOpts = `<option value="10m">10 minutes</option>
      <option value="1h">1 hour</option>
      <option value="6h">6 hours</option>
      <option value="1d" selected>1 day</option>
      <option value="7d">7 days</option>
      <option value="30d">30 days</option>
      <option value="90d">90 days</option>
      <option value="1y">1 year</option>
      <option value="never">Never</option>`

	// customCSS is the shared custom stylesheet applied on every page.
	customCSS = `<style>
:root {
  --pbin-radius-lg: 8px;
  --pbin-radius-md: 6px;
  --pbin-radius-sm: 4px;
  --pbin-success-bg: #ecfdf5;
  --pbin-success-border: #6ee7b7;
  --pbin-success-text: #065f46;
  --pbin-error-bg: #fef2f2;
  --pbin-error-border: #fca5a5;
  --pbin-error-text: #991b1b;
  --pbin-drop-border: #cbd5e1;
  --pbin-drop-hover-border: #6366f1;
  --pbin-drop-hover-bg: #eef2ff;
  --pbin-drop-icon: #94a3b8;
  --pbin-muted: #64748b;
  --pbin-surface: #f8fafc;
  --pbin-surface-border: #e2e8f0;
}
@media(prefers-color-scheme:dark){
:root {
  --pbin-success-bg: #052e16;
  --pbin-success-border: #166534;
  --pbin-success-text: #86efac;
  --pbin-error-bg: #450a0a;
  --pbin-error-border: #7f1d1d;
  --pbin-error-text: #fca5a5;
  --pbin-drop-border: #475569;
  --pbin-drop-hover-border: #818cf8;
  --pbin-drop-hover-bg: #1e1b4b;
  --pbin-drop-icon: #64748b;
  --pbin-muted: #94a3b8;
  --pbin-surface: #1e293b;
  --pbin-surface-border: #334155;
}}
body{margin:0;padding:0}
.container{max-width:720px;margin:0 auto;padding:0 1.25rem}
@media(min-width:1024px){.container{max-width:800px}}
@media(max-width:600px){.container{padding:0 1rem}.form-controls{grid-template-columns:1fr}}
nav{padding:.75rem 0;margin-bottom:1.5rem;border-bottom:1px solid var(--pbin-surface-border)}
nav .nav-inner{display:flex;justify-content:space-between;align-items:center;max-width:720px;margin:0 auto;padding:0 1.25rem}
@media(min-width:1024px){nav .nav-inner{max-width:800px}}
nav .brand{font-size:1.25rem;font-weight:700;text-decoration:none;color:inherit;letter-spacing:-.02em}
nav .nav-links{display:flex;gap:.25rem;list-style:none;margin:0;padding:0}
nav .nav-links a{padding:.4rem .85rem;border-radius:var(--pbin-radius-md);text-decoration:none;font-size:.9rem;font-weight:500;color:var(--pbin-muted);transition:background .15s,color .15s}
nav .nav-links a:hover{background:var(--pbin-surface);color:inherit}
nav .nav-links a[aria-current="page"]{background:var(--pbin-surface);color:inherit;font-weight:600}
footer.site-footer{margin-top:3rem;padding:1.5rem 0;border-top:1px solid var(--pbin-surface-border);text-align:center;font-size:.8rem;color:var(--pbin-muted)}
main.container{padding-top:1rem;padding-bottom:2rem}
#drop-zone{border:2px dashed var(--pbin-drop-border);border-radius:var(--pbin-radius-lg);padding:2.5rem 1.5rem;text-align:center;cursor:pointer;transition:border-color .2s,background .2s}
#drop-zone:hover{border-color:var(--pbin-drop-hover-border)}
#drop-zone.over{border-style:solid;border-color:var(--pbin-drop-hover-border);background:var(--pbin-drop-hover-bg)}
#drop-zone .drop-icon{display:block;margin:0 auto .75rem;color:var(--pbin-drop-icon)}
#drop-zone .drop-icon svg{width:48px;height:48px}
#drop-zone p{margin:0;font-size:.95rem;color:var(--pbin-muted)}
#drop-zone .drop-hint{font-size:.8rem;color:var(--pbin-muted);margin-top:.35rem}
#file-list{margin-top:.75rem}
#file-list .file-item{display:flex;align-items:center;justify-content:space-between;padding:.4rem .75rem;background:var(--pbin-surface);border:1px solid var(--pbin-surface-border);border-radius:var(--pbin-radius-sm);margin-bottom:.35rem;font-size:.85rem}
#file-list .file-item .file-name{font-weight:500;word-break:break-all}
#file-list .file-item .file-size{color:var(--pbin-muted);white-space:nowrap;margin-left:.75rem}
#file-list .file-item .file-remove{background:none;border:none;color:var(--pbin-error-text);cursor:pointer;padding:0 .25rem;font-size:1.1rem;line-height:1}
.form-controls{display:grid;grid-template-columns:1fr 1fr;gap:0 1.25rem;margin-top:1rem}
.form-controls label{margin-bottom:0}
.form-controls .full-width{grid-column:1/-1}
.result-card{margin-top:1.5rem;padding:1.25rem 1.5rem;background:var(--pbin-success-bg);border:1px solid var(--pbin-success-border);border-radius:var(--pbin-radius-lg)}
.result-card .result-title{display:flex;align-items:center;gap:.5rem;margin:0 0 1rem;font-size:1rem;color:var(--pbin-success-text);font-weight:600}
.result-card .result-title svg{width:20px;height:20px;flex-shrink:0}
.url-group{margin-bottom:.75rem}
.url-group .url-label{font-size:.75rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em;color:var(--pbin-success-text);margin-bottom:.25rem}
.url-row{display:flex;align-items:center;gap:.5rem}
.url-row code{flex:1;font-size:.85rem;word-break:break-all;padding:.4rem .6rem;background:rgba(0,0,0,.05);border-radius:var(--pbin-radius-sm);font-family:ui-monospace,monospace}
@media(prefers-color-scheme:dark){.url-row code{background:rgba(255,255,255,.08)}}
.url-row button[data-copy]{font-size:.75rem;padding:.3rem .65rem;border-radius:var(--pbin-radius-sm);border:1px solid var(--pbin-success-border);background:transparent;color:var(--pbin-success-text);cursor:pointer;white-space:nowrap;font-weight:500;transition:background .15s}
.url-row button[data-copy]:hover{background:rgba(0,0,0,.05)}
.result-card .result-meta{font-size:.85rem;color:var(--pbin-success-text);margin-top:.75rem}
.error-banner{margin-top:1.5rem;padding:1rem 1.25rem;background:var(--pbin-error-bg);border:1px solid var(--pbin-error-border);border-radius:var(--pbin-radius-lg);color:var(--pbin-error-text);display:flex;align-items:center;justify-content:space-between}
.error-banner .error-dismiss{background:none;border:none;color:var(--pbin-error-text);cursor:pointer;font-size:1.2rem;padding:0 .25rem;line-height:1;opacity:.7}
.error-banner .error-dismiss:hover{opacity:1}
.hidden{display:none}
</style>`

	// uploadIcon is an inline SVG used in the drop zone.
	uploadIcon = `<span class="drop-icon"><svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5m-13.5-9L12 3m0 0l4.5 4.5M12 3v13.5"/></svg></span>`

	// checkIcon is used in the result card title.
	checkIcon = `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>`
)

// navBarHTML returns the nav bar with the active page marked.
// activePage should be "file", "paste", or "bucket".
func navBarHTML(activePage string) string {
	uploadAttr, pasteAttr := "", ""
	switch activePage {
	case "upload":
		uploadAttr = ` aria-current="page"`
	case "paste":
		pasteAttr = ` aria-current="page"`
	}
	return fmt.Sprintf(`<nav>
  <div class="nav-inner">
    <a class="brand" href="/">pbin</a>
    <ul class="nav-links">
      <li><a href="/"%s>Upload</a></li>
      <li><a href="/paste"%s>Paste</a></li>
    </ul>
  </div>
</nav>`, uploadAttr, pasteAttr)
}

// viewNavBarHTML returns the nav bar for view pages (no active page).
func viewNavBarHTML() string {
	return navBarHTML("")
}

const footerHTML = `<footer class="site-footer"><div class="container">pbin v1.0</div></footer>`

// Home handles GET / — renders the unified upload form.
// Accepts one or multiple files. 1 file → file share, 2+ files → bucket.
func (h *UIHandler) Home(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiCSP)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pbin — Upload</title>
<link rel="stylesheet" href="%s">
%s
</head>
<body>
%s
<main class="container">
<h2>Upload Files</h2>
<form id="upload-form">
  <div id="drop-zone">
    %s
    <p>Drag &amp; drop files here, or click to select</p>
    <p class="drop-hint">One file = shareable link &middot; Multiple files = download bundle</p>
    <input type="file" id="file-input" name="file" multiple style="display:none">
  </div>
  <div id="file-list"></div>
  <div class="form-controls">
    <label for="expiry">Expiry
      <select id="expiry" name="expiry">%s</select>
    </label>
    <label for="password">Password (optional)
      <input type="password" id="password" name="password" placeholder="Leave blank for no password">
    </label>
    <label class="full-width">
      <input type="checkbox" id="one-use" name="one_use" value="1">
      One-use (disappears after first download)
    </label>
  </div>
  <button type="submit">Upload</button>
</form>
<div id="result" class="result-card hidden">
  <p class="result-title">%s<span id="result-heading">Uploaded!</span></p>
  <div class="result-meta" id="file-count-info" style="display:none"></div>
  <div class="url-group">
    <div class="url-label">Share URL</div>
    <div class="url-row"><code id="share-url"></code><button data-copy="">Copy</button></div>
  </div>
  <div class="url-group" id="embed-group" style="display:none">
    <div class="url-label">Embed URL</div>
    <div class="url-row"><code id="embed-url"></code><button data-copy="">Copy</button></div>
  </div>
  <div class="url-group">
    <div class="url-label">Delete URL</div>
    <div class="url-row"><code id="delete-url"></code><button data-copy="">Copy</button></div>
  </div>
  <div class="result-meta" id="expiry-info"></div>
</div>
<div id="error-msg" class="error-banner hidden"><span></span><button class="error-dismiss" onclick="this.parentElement.classList.add('hidden')">&times;</button></div>
</main>
%s
<script>
(function(){
  var dropZone = document.getElementById('drop-zone');
  var fileInput = document.getElementById('file-input');
  var fileList = document.getElementById('file-list');
  var selectedFiles = [];

  dropZone.addEventListener('click', function(){ fileInput.click(); });
  dropZone.addEventListener('dragover', function(e){ e.preventDefault(); dropZone.classList.add('over'); });
  dropZone.addEventListener('dragleave', function(){ dropZone.classList.remove('over'); });
  dropZone.addEventListener('drop', function(e){
    e.preventDefault();
    dropZone.classList.remove('over');
    selectedFiles = Array.from(e.dataTransfer.files);
    renderFileList();
  });
  fileInput.addEventListener('change', function(){
    selectedFiles = Array.from(fileInput.files);
    renderFileList();
  });

  function renderFileList(){
    if(selectedFiles.length === 0){ fileList.innerHTML = ''; return; }
    var html = '';
    selectedFiles.forEach(function(f, i){
      html += '<div class="file-item"><span class="file-name">'+escapeHtml(f.name)+'</span><span class="file-size">'+humanSize(f.size)+'</span><button class="file-remove" data-idx="'+i+'" type="button">&times;</button></div>';
    });
    fileList.innerHTML = html;
    fileList.querySelectorAll('.file-remove').forEach(function(btn){
      btn.addEventListener('click', function(){
        selectedFiles.splice(parseInt(btn.getAttribute('data-idx')),1);
        renderFileList();
      });
    });
  }

  function escapeHtml(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}
  function humanSize(n){
    if(n>=1073741824) return (n/1073741824).toFixed(1)+' GB';
    if(n>=1048576) return (n/1048576).toFixed(1)+' MB';
    if(n>=1024) return (n/1024).toFixed(1)+' KB';
    return n+' B';
  }

  document.getElementById('upload-form').addEventListener('submit', function(e){
    e.preventDefault();
    var form = e.target;
    if(selectedFiles.length === 0){
      showError('Please select at least one file.');
      return;
    }

    var isBucket = selectedFiles.length > 1;
    var fd = new FormData();
    if(isBucket){
      selectedFiles.forEach(function(f){ fd.append('file', f); });
    } else {
      fd.append('file', selectedFiles[0]);
    }
    fd.append('expiry', form.expiry.value);
    fd.append('password', form.password.value);
    fd.append('one_use', form['one_use'].checked ? '1' : '0');

    var btn = form.querySelector('button[type=submit]');
    btn.setAttribute('aria-busy','true');
    btn.disabled = true;

    var url = isBucket ? '/api/upload?type=bucket' : '/api/upload';
    fetch(url, {method:'POST', body: fd})
      .then(function(resp){ return resp.json().then(function(data){ return {status: resp.status, data: data}; }); })
      .then(function(r){
        btn.removeAttribute('aria-busy');
        btn.disabled = false;
        if(r.status === 201){
          showResult(r.data, isBucket);
        } else {
          showError(r.data.error || 'Upload failed.');
        }
      })
      .catch(function(){
        btn.removeAttribute('aria-busy');
        btn.disabled = false;
        showError('Network error. Please try again.');
      });
  });

  function showResult(data, isBucket){
    document.getElementById('error-msg').classList.add('hidden');
    document.getElementById('result-heading').textContent = isBucket
      ? 'Bucket uploaded!'
      : 'File uploaded!';
    var shareEl = document.getElementById('share-url');
    var deleteEl = document.getElementById('delete-url');
    shareEl.textContent = data.url;
    shareEl.nextElementSibling.setAttribute('data-copy', data.url);
    deleteEl.textContent = data.delete_url;
    deleteEl.nextElementSibling.setAttribute('data-copy', data.delete_url);
    document.getElementById('expiry-info').textContent = data.expires_at ? 'Expires: ' + data.expires_at : 'Never expires';
    // Bucket: show file count
    var countEl = document.getElementById('file-count-info');
    if(isBucket && data.file_count){
      countEl.textContent = data.file_count + ' file(s) uploaded';
      countEl.style.display = '';
    } else {
      countEl.style.display = 'none';
    }
    // Single file image: show embed link
    var embedGroup = document.getElementById('embed-group');
    if(!isBucket && data.is_image){
      var embedEl = document.getElementById('embed-url');
      var embedURL = data.url + '/info';
      embedEl.textContent = embedURL;
      embedEl.nextElementSibling.setAttribute('data-copy', embedURL);
      embedGroup.style.display = '';
    } else {
      embedGroup.style.display = 'none';
    }
    document.getElementById('result').classList.remove('hidden');
    initCopyButtons();
  }
  function showError(msg){
    document.getElementById('result').classList.add('hidden');
    var el = document.getElementById('error-msg');
    el.querySelector('span').textContent = msg;
    el.classList.remove('hidden');
  }
  function initCopyButtons(){
    document.querySelectorAll('[data-copy]').forEach(function(btn){
      btn.onclick = function(){
        navigator.clipboard.writeText(btn.getAttribute('data-copy'));
        var orig = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(function(){ btn.textContent = orig; }, 1500);
      };
    });
  }
})();
</script>
</body>
</html>`, picoCSS, customCSS, navBarHTML("upload"), uploadIcon, expiryOpts, checkIcon, footerHTML)
}

// Paste handles GET /paste — renders the paste creation form.
func (h *UIHandler) Paste(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiCSP)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pbin — New Paste</title>
<link rel="stylesheet" href="%s">
%s
<style>
#content{font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:.9rem;min-height:50vh;resize:vertical}
</style>
</head>
<body>
%s
<main class="container">
<h2>Create a Paste</h2>
<form id="paste-form">
  <textarea id="content" name="content" rows="20" required placeholder="Paste your text here..."></textarea>
  <div class="form-controls">
    <label for="title">Title (optional)
      <input type="text" id="title" name="title" placeholder="Untitled">
    </label>
    <label for="lang">Language
      <select id="lang" name="lang">
        <option value="plaintext">Plaintext</option>
        <option value="bash">Bash</option>
        <option value="c">C</option>
        <option value="cpp">C++</option>
        <option value="css">CSS</option>
        <option value="go">Go</option>
        <option value="html">HTML</option>
        <option value="java">Java</option>
        <option value="javascript">JavaScript</option>
        <option value="json">JSON</option>
        <option value="markdown">Markdown</option>
        <option value="python">Python</option>
        <option value="ruby">Ruby</option>
        <option value="rust">Rust</option>
        <option value="sql">SQL</option>
        <option value="typescript">TypeScript</option>
        <option value="yaml">YAML</option>
      </select>
    </label>
    <label for="paste-expiry">Expiry
      <select id="paste-expiry" name="expiry">%s</select>
    </label>
    <label for="paste-password">Password (optional)
      <input type="password" id="paste-password" name="password" placeholder="Leave blank for no password">
    </label>
    <label class="full-width">
      <input type="checkbox" id="paste-one-use" name="one_use">
      One-use (paste disappears after first view)
    </label>
  </div>
  <button type="submit">Create Paste</button>
</form>
<div id="result" class="result-card hidden">
  <p class="result-title">%sPaste created!</p>
  <div class="url-group">
    <div class="url-label">Share URL</div>
    <div class="url-row"><code id="share-url"></code><button data-copy="">Copy</button></div>
  </div>
  <div class="url-group">
    <div class="url-label">Delete URL</div>
    <div class="url-row"><code id="delete-url"></code><button data-copy="">Copy</button></div>
  </div>
  <div class="result-meta" id="expiry-info"></div>
</div>
<div id="error-msg" class="error-banner hidden"><span></span><button class="error-dismiss" onclick="this.parentElement.classList.add('hidden')">&times;</button></div>
</main>
%s
<script>
(function(){
  document.getElementById('paste-form').addEventListener('submit', function(e){
    e.preventDefault();
    var form = e.target;
    var payload = {
      content: form.content.value,
      title: form.title.value,
      lang: form.lang.value,
      expiry: form.expiry.value,
      password: form.password.value,
      one_use: form['one_use'].checked
    };
    var btn = form.querySelector('button[type=submit]');
    btn.setAttribute('aria-busy','true');
    btn.disabled = true;

    fetch('/api/paste', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload)
    })
      .then(function(resp){ return resp.json().then(function(data){ return {status: resp.status, data: data}; }); })
      .then(function(r){
        btn.removeAttribute('aria-busy');
        btn.disabled = false;
        if(r.status === 201){
          showResult(r.data);
        } else {
          showError(r.data.error || 'Paste creation failed.');
        }
      })
      .catch(function(){
        btn.removeAttribute('aria-busy');
        btn.disabled = false;
        showError('Network error. Please try again.');
      });
  });

  function showResult(data){
    document.getElementById('error-msg').classList.add('hidden');
    var shareEl = document.getElementById('share-url');
    var deleteEl = document.getElementById('delete-url');
    shareEl.textContent = data.url;
    shareEl.nextElementSibling.setAttribute('data-copy', data.url);
    deleteEl.textContent = data.delete_url;
    deleteEl.nextElementSibling.setAttribute('data-copy', data.delete_url);
    document.getElementById('expiry-info').textContent = data.expires_at ? 'Expires: ' + data.expires_at : 'Never expires';
    document.getElementById('result').classList.remove('hidden');
    initCopyButtons();
  }
  function showError(msg){
    document.getElementById('result').classList.add('hidden');
    var el = document.getElementById('error-msg');
    el.querySelector('span').textContent = msg;
    el.classList.remove('hidden');
  }
  function initCopyButtons(){
    document.querySelectorAll('[data-copy]').forEach(function(btn){
      btn.onclick = function(){
        navigator.clipboard.writeText(btn.getAttribute('data-copy'));
        var orig = btn.textContent;
        btn.textContent = 'Copied!';
        setTimeout(function(){ btn.textContent = orig; }, 1500);
      };
    });
  }
})();
</script>
</body>
</html>`, picoCSS, customCSS, navBarHTML("paste"), expiryOpts, checkIcon, footerHTML)
}

// Bucket handles GET /bucket — redirects to / (unified upload page).
func (h *UIHandler) Bucket(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

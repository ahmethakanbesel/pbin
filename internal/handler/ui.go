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
	uiCSPHome   = "default-src 'none'; script-src 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'unsafe-inline' https://cdn.jsdelivr.net; form-action 'self'; connect-src 'self'"
	uiCSPPaste  = "default-src 'none'; script-src 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'unsafe-inline' https://cdn.jsdelivr.net; form-action 'self'; connect-src 'self'"
	uiCSPBucket = "default-src 'none'; script-src 'unsafe-inline' https://cdn.jsdelivr.net; style-src 'unsafe-inline' https://cdn.jsdelivr.net; form-action 'self'; connect-src 'self'"
	expiryOpts  = `<option value="10m">10 minutes</option>
      <option value="1h">1 hour</option>
      <option value="6h">6 hours</option>
      <option value="1d" selected>1 day</option>
      <option value="7d">7 days</option>
      <option value="30d">30 days</option>
      <option value="90d">90 days</option>
      <option value="1y">1 year</option>
      <option value="never">Never</option>`
	navBar = `<nav>
  <ul><li><strong>pbin</strong></li></ul>
  <ul>
    <li><a href="/">File</a></li>
    <li><a href="/paste">Paste</a></li>
    <li><a href="/bucket">Bucket</a></li>
  </ul>
</nav>`
	dropZoneStyle = `<style>
#drop-zone{border:2px dashed #aaa;border-radius:6px;padding:2rem;text-align:center;cursor:pointer;color:#666;transition:border-color .2s,background .2s}
#drop-zone.over{border-color:#0066cc;background:#e8f0fe;color:#0066cc}
#file-list{margin-top:.5rem;font-size:.85rem;color:#555}
#result{margin-top:1rem;padding:1rem;background:#f0faf0;border:1px solid #9dc99d;border-radius:4px}
#error-msg{margin-top:1rem;padding:.75rem 1rem;background:#fff0f0;border:1px solid #f5a0a0;border-radius:4px;color:#c00}
.url-row{display:flex;align-items:center;gap:.5rem;margin:.25rem 0}
.url-row span{font-family:monospace;font-size:.9rem;word-break:break-all}
.hidden{display:none}
</style>`
)

// Home handles GET / — renders the file upload form.
func (h *UIHandler) Home(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiCSPHome)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pbin — File Upload</title>
<link rel="stylesheet" href="%s">
%s
</head>
<body>
%s
<main>
<h2>Upload a File</h2>
<form id="upload-form">
  <div id="drop-zone">
    <p>Drag &amp; drop a file here, or click to select</p>
    <input type="file" id="file-input" name="file" style="display:none">
  </div>
  <div id="file-list"></div>
  <label for="expiry">Expiry
    <select id="expiry" name="expiry">%s</select>
  </label>
  <label for="password">Password (optional)
    <input type="password" id="password" name="password" placeholder="Leave blank for no password">
  </label>
  <label>
    <input type="checkbox" id="one-use" name="one_use" value="1">
    One-use (file disappears after first download)
  </label>
  <button type="submit">Upload</button>
</form>
<div id="result" class="hidden">
  <strong>File uploaded!</strong>
  <p>Share URL:</p>
  <div class="url-row"><span id="share-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p>Delete URL:</p>
  <div class="url-row"><span id="delete-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p id="expiry-info"></p>
</div>
<div id="error-msg" class="hidden"></div>
</main>
<script>
(function(){
  var dropZone = document.getElementById('drop-zone');
  var fileInput = document.getElementById('file-input');
  var fileList = document.getElementById('file-list');

  dropZone.addEventListener('click', function(){ fileInput.click(); });
  dropZone.addEventListener('dragover', function(e){ e.preventDefault(); dropZone.classList.add('over'); });
  dropZone.addEventListener('dragleave', function(){ dropZone.classList.remove('over'); });
  dropZone.addEventListener('drop', function(e){
    e.preventDefault();
    dropZone.classList.remove('over');
    var files = e.dataTransfer.files;
    if(files.length > 0){
      fileInput.files = files;
      showFileNames(files);
    }
  });
  fileInput.addEventListener('change', function(){ showFileNames(fileInput.files); });

  function showFileNames(files){
    var names = [];
    for(var i=0;i<files.length;i++){ names.push(files[i].name); }
    fileList.textContent = 'Selected: ' + names.join(', ');
  }

  document.getElementById('upload-form').addEventListener('submit', function(e){
    e.preventDefault();
    var form = e.target;
    if(!fileInput.files || fileInput.files.length === 0){
      showError('Please select a file.');
      return;
    }
    var fd = new FormData();
    fd.append('file', fileInput.files[0]);
    fd.append('expiry', form.expiry.value);
    fd.append('password', form.password.value);
    fd.append('one_use', form['one_use'].checked ? '1' : '0');

    fetch('/api/upload', {method:'POST', body: fd})
      .then(function(resp){ return resp.json().then(function(data){ return {status: resp.status, data: data}; }); })
      .then(function(r){
        if(r.status === 201){
          showResult(r.data);
        } else {
          showError(r.data.error || 'Upload failed.');
        }
      })
      .catch(function(){ showError('Network error. Please try again.'); });
  });

  function showResult(data){
    document.getElementById('error-msg').classList.add('hidden');
    document.getElementById('share-url').textContent = data.url;
    document.getElementById('delete-url').textContent = data.delete_url;
    document.getElementById('expiry-info').textContent = data.expires_at ? 'Expires: ' + data.expires_at : 'Never expires';
    document.getElementById('result').classList.remove('hidden');
  }
  function showError(msg){
    document.getElementById('result').classList.add('hidden');
    var el = document.getElementById('error-msg');
    el.textContent = msg;
    el.classList.remove('hidden');
  }
})();
</script>
</body>
</html>`, picoCSS, dropZoneStyle, navBar, expiryOpts)
}

// Paste handles GET /paste — renders the paste creation form.
func (h *UIHandler) Paste(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiCSPPaste)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pbin — New Paste</title>
<link rel="stylesheet" href="%s">
%s
</head>
<body>
%s
<main>
<h2>Create a Paste</h2>
<form id="paste-form">
  <label for="title">Title (optional)
    <input type="text" id="title" name="title" placeholder="Untitled">
  </label>
  <label for="content">Content
    <textarea id="content" name="content" rows="16" required placeholder="Paste your text here..."></textarea>
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
  <label>
    <input type="checkbox" id="paste-one-use" name="one_use">
    One-use (paste disappears after first view)
  </label>
  <button type="submit">Create Paste</button>
</form>
<div id="result" class="hidden">
  <strong>Paste created!</strong>
  <p>Share URL:</p>
  <div class="url-row"><span id="share-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p>Delete URL:</p>
  <div class="url-row"><span id="delete-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p id="expiry-info"></p>
</div>
<div id="error-msg" class="hidden"></div>
</main>
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
    fetch('/api/paste', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload)
    })
      .then(function(resp){ return resp.json().then(function(data){ return {status: resp.status, data: data}; }); })
      .then(function(r){
        if(r.status === 201){
          showResult(r.data);
        } else {
          showError(r.data.error || 'Paste creation failed.');
        }
      })
      .catch(function(){ showError('Network error. Please try again.'); });
  });

  function showResult(data){
    document.getElementById('error-msg').classList.add('hidden');
    document.getElementById('share-url').textContent = data.url;
    document.getElementById('delete-url').textContent = data.delete_url;
    document.getElementById('expiry-info').textContent = data.expires_at ? 'Expires: ' + data.expires_at : 'Never expires';
    document.getElementById('result').classList.remove('hidden');
  }
  function showError(msg){
    document.getElementById('result').classList.add('hidden');
    var el = document.getElementById('error-msg');
    el.textContent = msg;
    el.classList.remove('hidden');
  }
})();
</script>
</body>
</html>`, picoCSS, dropZoneStyle, navBar, expiryOpts)
}

// Bucket handles GET /bucket — renders the multi-file bucket upload form.
func (h *UIHandler) Bucket(w http.ResponseWriter, r *http.Request) {
	securityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiCSPBucket)

	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>pbin — Bucket Upload</title>
<link rel="stylesheet" href="%s">
%s
</head>
<body>
%s
<main>
<h2>Upload a Bucket</h2>
<p>Upload multiple files as a shareable bundle.</p>
<form id="bucket-form">
  <div id="drop-zone">
    <p>Drag &amp; drop files here, or click to select</p>
    <input type="file" id="file-input" name="file" multiple style="display:none">
  </div>
  <div id="file-list"></div>
  <label for="bucket-expiry">Expiry
    <select id="bucket-expiry" name="expiry">%s</select>
  </label>
  <label for="bucket-password">Password (optional)
    <input type="password" id="bucket-password" name="password" placeholder="Leave blank for no password">
  </label>
  <label>
    <input type="checkbox" id="bucket-one-use" name="one_use" value="1">
    One-use (bucket disappears after first download)
  </label>
  <button type="submit">Upload Bucket</button>
</form>
<div id="result" class="hidden">
  <strong>Bucket uploaded!</strong>
  <p id="file-count-info"></p>
  <p>Share URL:</p>
  <div class="url-row"><span id="share-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p>Delete URL:</p>
  <div class="url-row"><span id="delete-url"></span><button onclick="navigator.clipboard.writeText(this.previousElementSibling.textContent)">Copy</button></div>
  <p id="expiry-info"></p>
</div>
<div id="error-msg" class="hidden"></div>
</main>
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
    showFileNames(selectedFiles);
  });
  fileInput.addEventListener('change', function(){
    selectedFiles = Array.from(fileInput.files);
    showFileNames(selectedFiles);
  });

  function showFileNames(files){
    if(files.length === 0){ fileList.textContent = ''; return; }
    var html = '<ul style="margin:.5rem 0;padding-left:1.2rem">';
    files.forEach(function(f){
      html += '<li>' + escapeHtml(f.name) + ' (' + humanSize(f.size) + ')</li>';
    });
    html += '</ul>';
    fileList.innerHTML = html;
  }

  function escapeHtml(s){
    return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
  }

  function humanSize(n){
    if(n >= 1073741824) return (n/1073741824).toFixed(1)+' GB';
    if(n >= 1048576) return (n/1048576).toFixed(1)+' MB';
    if(n >= 1024) return (n/1024).toFixed(1)+' KB';
    return n+' B';
  }

  document.getElementById('bucket-form').addEventListener('submit', function(e){
    e.preventDefault();
    var form = e.target;
    if(selectedFiles.length === 0){
      showError('Please select at least one file.');
      return;
    }
    var fd = new FormData();
    selectedFiles.forEach(function(f){ fd.append('file', f); });
    fd.append('expiry', form.expiry.value);
    fd.append('password', form.password.value);
    fd.append('one_use', form['one_use'].checked ? '1' : '0');

    fetch('/api/upload?type=bucket', {method:'POST', body: fd})
      .then(function(resp){ return resp.json().then(function(data){ return {status: resp.status, data: data}; }); })
      .then(function(r){
        if(r.status === 201){
          showResult(r.data);
        } else {
          showError(r.data.error || 'Bucket upload failed.');
        }
      })
      .catch(function(){ showError('Network error. Please try again.'); });
  });

  function showResult(data){
    document.getElementById('error-msg').classList.add('hidden');
    document.getElementById('share-url').textContent = data.url;
    document.getElementById('delete-url').textContent = data.delete_url;
    document.getElementById('expiry-info').textContent = data.expires_at ? 'Expires: ' + data.expires_at : 'Never expires';
    document.getElementById('file-count-info').textContent = data.file_count + ' file(s) uploaded';
    document.getElementById('result').classList.remove('hidden');
  }
  function showError(msg){
    document.getElementById('result').classList.add('hidden');
    var el = document.getElementById('error-msg');
    el.textContent = msg;
    el.classList.remove('hidden');
  }
})();
</script>
</body>
</html>`, picoCSS, dropZoneStyle, navBar, expiryOpts)
}

package handler

import "fmt"

// sharedJS contains copy-to-clipboard utilities used across all pages.
const sharedJS = `
function copyText(text){
  if(navigator.clipboard&&navigator.clipboard.writeText){
    navigator.clipboard.writeText(text).catch(function(){fallbackCopy(text);});
  } else {fallbackCopy(text);}
}
function fallbackCopy(text){
  var ta=document.createElement('textarea');
  ta.value=text;ta.style.position='fixed';ta.style.opacity='0';
  document.body.appendChild(ta);ta.select();
  try{document.execCommand('copy');}catch(e){}
  document.body.removeChild(ta);
}
function initCopyButtons(){
  document.querySelectorAll('[data-copy]').forEach(function(btn){
    if(btn._copyBound) return;
    btn._copyBound=true;
    btn.addEventListener('click', function(){
      copyText(btn.getAttribute('data-copy'));
      var orig=btn.textContent;
      btn.textContent='Copied!';
      setTimeout(function(){btn.textContent=orig;},1500);
    });
  });
}
`

// pageHead returns everything from <!DOCTYPE html> through the closing </head>.
// extraHead items are inserted after customCSS (for page-specific styles).
func pageHead(title, csp string, extraHead ...string) string {
	extra := ""
	for _, e := range extraHead {
		extra += e
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="theme-color" content="#6366f1">
<title>%s</title>
<meta http-equiv="Content-Security-Policy" content="%s">
<link rel="stylesheet" href="%s">
%s
%s
</head>`, title, csp, picoCSS, customCSS, extra)
}

// pageOpen returns the opening <body>, nav bar, and <main class="container">.
func pageOpen(activePage string) string {
	return fmt.Sprintf("<body>\n%s\n<main class=\"container\">", navBarHTML(activePage))
}

// pageClose returns the closing </main>, footer, scripts, and closing tags.
// Scripts are inserted raw — caller wraps in scriptTag if needed.
func pageClose(scripts ...string) string {
	s := "</main>\n" + footerHTML
	for _, sc := range scripts {
		s += "\n" + sc
	}
	s += "\n</body>\n</html>"
	return s
}

// scriptTag wraps content in a <script> tag. If nonce is non-empty, it is added as an attribute.
func scriptTag(content, nonce string) string {
	if nonce != "" {
		return fmt.Sprintf("<script nonce=\"%s\">\n%s\n</script>", nonce, content)
	}
	return fmt.Sprintf("<script>\n%s\n</script>", content)
}

(() => { const t = document.title; const bodyText = document.body ? document.body.innerText.slice(0, 800) : ""; return JSON.stringify({ title: t, body: bodyText }); })()

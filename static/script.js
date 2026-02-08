// Handle relative page links within the same version
document.querySelectorAll('main a[href$=".md"]').forEach(a => {
	const href = a.getAttribute('href');
	if (!href.startsWith('http')) {
		a.href = href.replace(/\.md$/, '.html');
	}
});

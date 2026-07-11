// Rewrite relative in-content *.md links to their generated .html pages.
// Absolute links (starting with "/" or a scheme) are left alone — e.g. the
// generated llms-full.md links point at real markdown files.
document.querySelectorAll('main a[href$=".md"]').forEach(a => {
	const href = a.getAttribute('href');
	if (!href.startsWith('http') && !href.startsWith('/')) {
		a.href = href.replace(/\.md$/, '.html');
	}
});

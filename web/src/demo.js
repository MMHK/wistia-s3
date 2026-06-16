import "highlight.js/styles/github-dark.min.css"
import hljs from 'highlight.js/lib/core';
import html from 'highlight.js/lib/languages/xml';
import ClipboardJS from "clipboard";

hljs.registerLanguage('xml', html);
new ClipboardJS(".copy-btn");
hljs.highlightAll();

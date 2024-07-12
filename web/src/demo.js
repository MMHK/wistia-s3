import "highlight.js/styles/github-dark.min.css"
import hljs from 'highlight.js/lib/core';
import html from 'highlight.js/lib/languages/xml';
import ClipboardJS from "clipboard";

const wrapper = document.querySelector("#code-block-1");
const raw = `
<div class="wistia_responsive_padding">
    <div class="wistia_responsive_wrapper">
        <div class="wistia_embed wistia_async_{{.HashId}} videoFoam=true playsinline=true" style="height:100%;width:100%">&nbsp;</div>
    </div>
</div>
<script type="text/javascript" src="{{.WistiaS3JSUrl}}"></script>`;

hljs.registerLanguage('xml', html);

wrapper.innerHTML = hljs.highlight(
  raw,
  { language: 'xml' }
).value

// Then register the languages you need

new ClipboardJS(".copy-btn");


hljs.highlightAll();

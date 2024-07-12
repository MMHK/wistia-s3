(function (e) {
  window.MEDIA_ENDPOINT = e.MEDIA_ENDPOINT || undefined;
  require("wistia-player-alternative/dist/js/main");
})({
  ...window,
  MEDIA_ENDPOINT: "{{.MediaEndPoint}}"
})

(function (e) {
  window.MEDIA_ENDPOINT = e.MEDIA_ENDPOINT || undefined;
  require("wistia-s3-player/dist/js/wistia-s3-player.min");
})({
  ...window,
  MEDIA_ENDPOINT: "{{.MediaEndPoint}}"
})

(function (e) {
  window.MEDIA_ENDPOINT = e.MEDIA_ENDPOINT || undefined;
  import("wistia-s3-player/dist/js/wistia-s3-player.min")
    .then((module) => {
      const init = module.default || module;
      const trackingID = "{{.TrackingID}}";
      init(trackingID);
    });
})({
  ...window,
  MEDIA_ENDPOINT: "{{.MediaEndPoint}}"
})

import wistiaPlayer from "wistia-s3-player/dist/js/wistia-s3-player.min";

(function (e) {
  window.MEDIA_ENDPOINT = e.MEDIA_ENDPOINT || undefined;
  const trackingID = "{{.TrackingID}}";
  wistiaPlayer(trackingID);
})({
  ...window,
  MEDIA_ENDPOINT: "{{.MediaEndPoint}}"
})

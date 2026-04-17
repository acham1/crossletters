// Service worker for Web Push notifications.
self.addEventListener("push", (event) => {
  const data = event.data ? event.data.json() : {};
  const title = data.title || "not-scrabble";
  const options = {
    body: data.body || "It's your turn!",
    icon: data.icon,
    data: { url: data.url },
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const url = event.notification.data?.url;
  if (url) {
    event.waitUntil(clients.openWindow(new URL(url, self.location.origin).href));
  }
});

self.addEventListener('push', event => {
  console.log('[Service Worker] Push Received.');
  console.log(`[Service Worker] Push had this data: "${event.data.text()}"`);

  const title = 'Goose, la red social libre';
  const options = {
    body: event.data.text(),
    badge: "https://goose.blue/avatar.png",
    icon: "https://goose.blue/avatar.png",
    vibrate: [50, 10, 200, 10, 200, 10, 200, 10, 200],
  };

  event.waitUntil(self.registration.showNotification(title, options));
  console.log(event);
});

self.addEventListener('notificationclick', (event) => {
  console.log('On notification click: ', event);
  event.notification.close();

  // This looks to see if the current is already open and
  // focuses if it is
  event.waitUntil(clients.matchAll({
    type: "window"
  }).then((clientList) => {
    for (const client of clientList) {
      if (client.url === '/' && 'focus' in client)
        return client.focus();
    }
    if (clients.openWindow)
      return clients.openWindow('/');
  }));
});


self.addEventListener('push', event => {
  console.log('[Service Worker] Push Received.');
  console.log(`[Service Worker] Push had this data: "${event.data.text()}"`);

  const title = 'Goose, la red social libre';
  const options = {
    body: event.data.text(),
    badge: "https://goose.blue/badge-monochrome.png",
    icon: "https://goose.blue/avatar.png",
    vibrate: [50, 10, 200, 10, 200, 10, 200, 10, 200],
    tag: 'social-interaction',
    renotify: true,
  };

  event.waitUntil(self.registration.showNotification(title, options));
  console.log(event);
});

self.addEventListener('notificationclick', (event) => {
  console.log('On notification click: ', event);
  console.log('this', this);
  event.notification.close();


  const examplePage = 'https://goose.blue/';

  // This looks to see if the current is already open and
  // focuses if it is
  event.waitUntil(clients.matchAll({
    type: "window",
    includeUncontrolled: true,
  }).then((clientList) => {
    for (const client of clientList) {
      console.log('client', client)
      if (client.url === examplePage) return client.focus();
    }
    return clients.openWindow(examplePage);
  }));
});


self.addEventListener('push', event => {
  console.log('[Service Worker] Push Received.');
  console.log(`[Service Worker] Push had this data: "${event.data.text()}"`);

  let title = 'Goose, la red social libre';
  let options = {
    badge: "https://goose.blue/badge-monochrome.png",
    icon: "https://goose.blue/logo-blue-200.png",
    vibrate: [50, 10, 200, 10, 200, 10, 200, 10, 200],
    tag: 'social-interaction',
    renotify: true,
  };

  try {
    let p = JSON.parse(event.data.text());
    options.data = p.data;
    if (p.title) title = p.title;
    if (p.icon) options.icon = p.icon;
    if (p.body) options.body = p.body;
  } catch {
    options.body = event.data.text();
  }

  event.waitUntil(self.registration.showNotification(title, options));
  console.log(event);
});

self.addEventListener('notificationclick', (event) => {
  console.log('On notification click: ', event);
  console.log('this', this);
  event.notification.close();

  let data = event.notification.data;

  let examplePage = 'https://goose.blue/';
  if (data && data.open) {
    examplePage = data.open;
  }

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


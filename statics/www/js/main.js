
const banner = `
   _____                      
  / ____|                     
 | |  __  ___   ___  ___  ___ 
 | | |_ |/ _ \\ / _ \\/ __|/ _ \\
 | |__| | (_) | (_) \\__ \\  __/
  \\_____|\\___/ \\___/|___/\\___|

You can contribute here: https://github.com/dataless-io/goose/

Have fun!!!!
`;
console.log(banner);

function id(s) {
    return document.getElementById(s);
}

function prettyDate(d) {
    let now = new Date();

    let delta = (now.getTime() - d.getTime()) / 1000;

    if (delta < 60) {
        return 'justo ahora';
    }

    if (delta < 3600) {
        return `hace ${(delta/60).toFixed()} minutos`;
    }

    if (delta < 86400) {
        return `hace ${(delta/3600).toFixed()} horas`;
    }

    return `${d.getDate()}/${d.getMonth()+1}/${d.getFullYear()} a las ${d.getHours()}:${d.getMinutes()}`
}

function compare( a, b ) {
    if ( a < b ){
        return -1;
    }
    if ( a > b ){
        return 1;
    }
    return 0;
}

function substrBytes(str, start, length) {
    var buf = new Buffer(str);
    return buf.slice(start, start+length).toString();
}

document.querySelectorAll("article.tweet").forEach(item => {

    const dom_author = item.querySelector('.author');
    const dom_message = item.querySelector('.message');

    // Add links to message
    try {
        const links = JSON.parse(item.dataset.links).sort((a,b) => {
            return compare(a.begin, b.begin);
        });
        // const message = item.dataset.message;
        const message = dom_message.textContent;
        dom_message.textContent = '';
        let last = 0;

        dom_mentions = document.createElement('div')
        dom_mentions.classList.add('mentions');

        links.push({type:'none'});
        links.forEach(link => {

            // add plain text:
            const span = document.createElement('span'); // probably should be a text node
            span.textContent = message.substring(last, link.begin);
            dom_message.appendChild(span);

            last = link.end;

            switch (link.type) {
                case 'handler': {
                    const a = document.createElement('a');
                    a.href = '/user/'+link.extra.handle;
                    a.textContent = message.substring(link.begin, link.end);
                    dom_message.appendChild(a)

                    // Add to mentions
                    const v = document.createElement('a');
                    v.href = '/user/'+link.extra.handle;
                    v.title = link.extra.nick;
                    const img = document.createElement('img');
                    img.src = link.extra.picture || '/avatar.png';
                    v.appendChild(img);
                    dom_mentions.appendChild(v);
                    break;
                }
                case 'hashtag': {
                    const a = document.createElement('a');
                    a.href = '/hashtag/'+link.text;
                    a.textContent = message.substring(link.begin, link.end);
                    dom_message.appendChild(a)
                    break;
                }
                case 'url': {
                    const a = document.createElement('a');
                    a.href = link.text;
                    a.textContent = message.substring(link.begin, link.end);
                    dom_message.appendChild(a)
                    break;
                }
                case 'code': {
                    const s1 = document.createElement('span');
                    s1.classList.add('code-surround');
                    s1.textContent = message.substring(link.begin, link.begin+1);
                    dom_message.appendChild(s1)
                    const a = document.createElement('code');
                    a.textContent = message.substring(link.begin+1, link.end-1);
                    dom_message.appendChild(a)
                    const s2 = document.createElement('span');
                    s2.classList.add('code-surround');
                    s2.textContent = message.substring(link.end-1, link.end);
                    dom_message.appendChild(s2)
                    break;
                }
                case 'none: {': {
                    // do nothing
                    break;
                }
            }


        });

        // // add plain text:
        // const span = document.createElement('span'); // probably should be a text node
        // span.textContent = message.substring(last);
        // dom_message.appendChild(span);
        if (dom_mentions.children.length) {
            item.appendChild(dom_mentions);
        }

    } catch (e) {
        //
    }

    // Pretty date:
    const dom_time = item.querySelector('time');
    const dom_date = item.querySelector('.date');
    // const dom_nickname = item.querySelector('.nickname');
    const honk_date = new Date(1000*dom_time.dataset.unix);
    dom_time.textContent = prettyDate(honk_date);
    dom_time.setAttribute('title', honk_date.toLocaleString());

    // Follow button:
    const dom_avatar = item.querySelector('a.avatar');
    const button_follow = document.createElement('button');
    button_follow.classList.add('button-follow');
    button_follow.textContent = '👣';
    button_follow.userId = dom_author.dataset.id;
    if (item.dataset.followed == 'true') {
        button_follow.classList.add('active');
    }
    button_follow.addEventListener('click', function () {

        let method = 'POST';
        if (this.classList.contains('active')) {
            method = 'DELETE';
            this.classList.remove('active');
        } else {
            this.classList.add('active');
        }

        fetch('/v0/users/'+encodeURIComponent(this.userId)+'/follow', {method:method, headers: {'X-Glue-Authentication':XAuthHeader}})
            .then(resp => {
                if (resp.status != 200) {
                    throw 'bad status code';
                }
            });
    }, true);
    item.insertBefore(button_follow, dom_avatar.nextSibling);

    // Footer buttons:
    const footer = document.createElement('footer');
    footer.classList.add('buttons');

    const button_comment = document.createElement('button');
    button_comment.classList.add('button');
    button_comment.textContent = 'Comment';
    // footer.appendChild(button_comment);

    const button_rehonk = document.createElement('button');
    button_rehonk.classList.add('button');
    button_rehonk.textContent = 'Rehonk';
    // footer.appendChild(button_rehonk);

    const button_like = document.createElement('button');
    button_like.classList.add('button');
    button_like.textContent = 'Like';
    // footer.appendChild(button_like);

    const button_share = document.createElement('button');
    button_share.classList.add('button');
    button_share.addEventListener('click', function() {
        window.location.href = 'https://twitter.com/intent/tweet?text='+encodeURIComponent(dom_message.textContent)+
        '&url='+encodeURIComponent('https://goose.blue'+dom_date.getAttribute('href'))+
        '&hashtags=gooseblue,cloneTweeter,openSource';
    }, true);
    button_share.setAttribute('title', 'Compartir');
    footer.appendChild(button_share);
    const icon_share = document.createElement('span');
    icon_share.classList.add('icon');
    icon_share.classList.add('icon-share');
    button_share.appendChild(icon_share);

    item.appendChild(footer);

});

function setupNotifications() {

    // const XAuthHeader = '{"user":{"email":"fulanez@gmail.com","id":"user-123","nick":"fulanez","picture":"/avatar.png","Xerror":"unauthorized"}}';
    let XAuthHeader = JSON.stringify({user});

    function sendSubscription(subscription) {
        let payload = JSON.stringify(subscription);
        fetch('/v0/push/register', {method:'POST', body: payload, headers: {'X-Glue-Authentication':XAuthHeader}});
    }

    function subscribe() {
        navigator.serviceWorker.ready
            .then(function(registration) {
                const vapidPublicKey = 'BEVpiHG9LmOtwnCLeiWcJMeUDbOWH5vhKX-Xss4F1qA3pWin7WvOF0-z906obEdRbrHZqRpZWRBhQMcjB744i_Y';
                return registration.pushManager.subscribe({
                    userVisibleOnly: true,
                    applicationServerKey: urlBase64ToUint8Array(vapidPublicKey),
                });
            })
            .then(function(subscription) {
                sendSubscription(subscription);
            })
            .catch(err => console.error(err));
    }

    function urlBase64ToUint8Array(base64String) {
        const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
        const base64 = (base64String + padding)
            .replace(/\-/g, '+')
            .replace(/_/g, '/');
        const rawData = window.atob(base64);
        return Uint8Array.from([...rawData].map(char => char.charCodeAt(0)));
    }

    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/web-push-worker.js?9');
        navigator.serviceWorker.ready
            .then(function(registration) {
                return registration.pushManager.getSubscription();
            })
            .then(function(subscription) {
                if (!subscription) {
                    subscribe();
                } else {
                    sendSubscription(subscription);
                }
            });
    }
}

let user = {};
let XAuthHeader = '';
const avatar = document.createElement('img');
avatar.setAttribute('src', '/avatar.png');
fetch('/auth/me').then(resp => {
    if (resp.status != 200) {
        throw 'bad status code';
    }
    return resp.json();
}).then(resp => {

    if (resp.error) {
        throw resp.error;
    }

    let authDiv = id('auth');

    user = resp;
    XAuthHeader = JSON.stringify({user});

    let linkUser = document.createElement('a');
    linkUser.classList.add('auth-link-user');
    linkUser.setAttribute('href', '/user/'+encodeURIComponent(resp.nick));
    authDiv.append(linkUser);

    let picture = document.createElement('img');
    picture.classList.add('auth-picture');
    picture.setAttribute('src', resp.picture);
    linkUser.append(picture);

    avatar.setAttribute('src', resp.picture);

    let nick = document.createElement('span');
    nick.classList.add('auth-nick');
    nick.textContent = resp.nick;
    linkUser.append(nick);

    let logout = document.createElement('a');
    logout.classList.add('rounded-button');
    logout.classList.add('rounded-button-outlined');
    logout.textContent = "Logout";
    logout.href = '/auth/logout';
    authDiv.append(logout);
    setupNotifications();
}).catch( reason => {
    console.log("AUTH:", reason);
    let login = document.createElement('a');
    login.classList.add('rounded-button');
    login.textContent = "Login";
    login.href = '/auth/login';
    id('auth').append(login);
});

function sendHonk(f, message, parentHonkId) {
    let body = {
        message: message,
    };
    if (parentHonkId) body.parent_honk_id = parentHonkId;

    fetch('/v0/publish', {method:'POST', body:JSON.stringify(body), headers: {'X-Glue-Authentication':XAuthHeader}})
    .then(resp => resp.json())
    .then(entry => {
        // todo: check status code, network error
        if (entry.error) {
            alert(entry.error.message);
            return
        }
        f(entry);
    })
    .catch(function (err) {
        alert(err);
    });
}

(function() {
    if (window.localStorage) {
        console.log('localStorage avaiable');
        return;
    }
    window.localStorage = {
        clear: function() {},
        getItem: function() {},
        removeItem: function() {},
        setItem: function() {},
    };
})();

(function() {
    let publish_dom = document.querySelector("#publish");
    if (!publish_dom) return;

    // avatar is created in global scope
    avatar.classList.add('avatar');
    publish_dom.appendChild(avatar);

    const text_input = document.createElement('textarea');
    text_input.setAttribute('rows', 4);
    text_input.placeholder = '¿Qué está pasando?';
    text_input.value = localStorage.getItem('new-honk');
    publish_dom.appendChild(text_input);

    const buttons = document.createElement('div');
    buttons.classList.add('buttons');
    publish_dom.appendChild(buttons);

    const MAX_LENGTH = 300;

    const counter = document.createElement('span');
    counter.classList.add('counter');
    buttons.appendChild(counter);

    const publish_button = document.createElement('button');
    publish_button.textContent = "Honk!";
    publish_button.setAttribute('disabled', true);
    buttons.appendChild(publish_button);

    update_counter = function() {
        if (text_input.value.length == 0) {
            publish_button.setAttribute('disabled', true);
            counter.textContent = '';
        } else {
            publish_button.removeAttribute('disabled');
            const remaining = MAX_LENGTH-text_input.value.length;
            counter.textContent = ''+(remaining);
            if (remaining >= 0) {
                counter.classList.remove('red');
                publish_button.removeAttribute('disabled');
            } else {
                counter.classList.add('red');
                publish_button.setAttribute('disabled', true);
            }
        }
    };

    text_input.addEventListener('keyup', update_counter, true);
    publish_button.addEventListener('click', function() {

        if (!user.id) {
            // todo: save text_input.value somwhere
            window.location.href = '/auth/login';
            return;
        }

        sendHonk(function (honk) {
            text_input.value = '';
            update_counter();
            localStorage.removeItem('new-honk');
            location.reload();
        }, text_input.value);
    }, true);
    text_input.addEventListener('keyup', function (){
        localStorage.setItem('new-honk', this.value);
    }, true);
})();

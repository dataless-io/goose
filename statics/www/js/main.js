
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

document.querySelectorAll("article.tweet").forEach(item => {

    // Pretty date:
    const dom_time = item.querySelector('time');
    const honk_date = new Date(1000*dom_time.dataset.unix);
    dom_time.textContent = prettyDate(honk_date);
    dom_time.setAttribute('title', honk_date.toLocaleString());

    // Footer buttons:
    const footer = document.createElement('footer');
    footer.classList.add('buttons');

    const button_comment = document.createElement('button');
    button_comment.classList.add('button');
    button_comment.textContent = 'Comment';
    footer.appendChild(button_comment);

    const button_rehonk = document.createElement('button');
    button_rehonk.classList.add('button');
    button_rehonk.textContent = 'Rehonk';
    footer.appendChild(button_rehonk);

    const button_like = document.createElement('button');
    button_like.classList.add('button');
    button_like.textContent = 'Like';
    footer.appendChild(button_like);

    const button_share = document.createElement('button');
    button_share.classList.add('button');
    button_share.textContent = 'Share';
    footer.appendChild(button_share);

    item.appendChild(footer);

});

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

    let picture = document.createElement('img');
    picture.classList.add('auth-picture');
    picture.setAttribute('src', resp.picture);
    authDiv.append(picture);

    avatar.setAttribute('src', resp.picture);

    let nick = document.createElement('span');
    nick.classList.add('auth-nick');
    nick.textContent = resp.nick;
    authDiv.append(nick);

    let logout = document.createElement('a');
    logout.classList.add('rounded-button');
    logout.classList.add('rounded-button-outlined');
    logout.textContent = "Logout";
    logout.href = '/auth/logout';
    authDiv.append(logout);
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


    let publish_dom = document.querySelector("#publish");

    // avatar is created in global scope
    avatar.classList.add('avatar');
    publish_dom.appendChild(avatar);

    const text_input = document.createElement('textarea');
    text_input.setAttribute('rows', 4);
    text_input.placeholder = '¿Qué está pasando?';
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
            console.log({honk});
            text_input.value = '';
            update_counter();
            location.reload();
        }, text_input.value);
    }, true);


})();


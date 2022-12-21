
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

    let nick = document.createElement('span');
    nick.classList.add('auth-nick');
    nick.textContent = resp.nick;
    authDiv.append(nick);

    let logout = document.createElement('a');
    logout.textContent = "Logout";
    logout.href = '/auth/logout';
    authDiv.append(logout);
}).catch( reason => {
    console.log("AUTH:", reason);
    let login = document.createElement('a');
    login.textContent = "Login";
    login.href = '/auth/login';
    id('auth').append(login);
});

# Goose

<div style="text-align: center;" align="center">
    <img src="statics/www/logo-200.jpg">
</div> 

Goose is a cheap clone of Tweeter to test [InceptionDB](https://github.com/fulldump/inceptiondb) from
users as developer.


## How to run

Golang is required. Just run:

```shell
make run
```

Or to develop the frontend:
```shell
HTTPADDR=:8080 STATICS=./statics/www/ make run
```

## Contribute

Feel free to contribute, you can improve the interface, API, the stream algorithm...

Just make a PR. The code should compile and be ready to work.

## Comming soon... or not

TOP 3:
* Join Date (for users)
* Re-honk
* Likes
  * Aggregate likes on each view
  * Gather counter and control unique users
  * Insert on activity
  * Insert on followers

REST:
* Split user data flows input (feed) and output (activity)
  * feed: mentions, followers, ai recommendations ...
  * activity: honks, re-honks, comments, likes
* Pagination (infinite scroll) on feed, activity...
* New honk button always visible (material style)
* Print stats
* Explore Groups/Rooms/Topics
* Comment
* Edit nick, picture
* Description
* Highlight http://links, @mentions, #hashtags
* Process hashtags: Stats, trends, navigation, api...
* Previews (parse meta tags of links, etc)
* Bottom bar
* Embed resouces from specific services (videos from youtube...)
* Share with goose (android traits)
* Delete honks
* Edit honks
* Notifications


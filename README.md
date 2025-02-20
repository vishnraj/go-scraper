# Purpose
Command-line tool to interact with HTML content retrieved from dynamic web pages  
Uses the chromedp library:  
https://github.com/chromedp/chromedp

# Usage

## General
```
Allows you to request data from dynamic web pages and interact with it

Usage:
  go-scraper [command]

Available Commands:
  fetch       Write the HTML content for the URL to stdout
  help        Help about any command
  watch       Watch URL(s) and take an action if criteria is met

Flags:
  -a, --agent string           User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --captcha_click_selector string            The selector element to click for the captcha box (default "div.g-recaptcha")
      --captcha_click_sleep int               Time (seconds) we sleep after a captcha click, to allow the captcha challenge to get loaded into the iframe (default 5)
      --captcha_iframe_wait_selector string   The selector element to wait for the captcha iframe (default "body > div:nth-child(6) > div:nth-child(4) > iframe")
      --captcha_wait_selector string             The selector element to wait for so we can load the captcha box (default "div.re-captcha")
      --config string          config file (default is $HOME/.go-scraper.yaml)
      --detect_access_denied   If access denied is encoutered, then we will take a counter action
      --detect_captcha_box                    If a captcha box is encoutered, then we will take a counter action
      --detect_notify_path                    If a desired notify path is encountered, for a given URL, perform notification action
      --error_dump             Dumps current page contents on error
      --error_location         Logs the current URL that we have arrived at on error
      --headless               Use headless shell
  -h, --help                   help for go-scraper
  --redis_dump                 Set this option for all dumps to go to the redis database that we connet to this app
  --redis_key_expiration int   The duration, in secondds that keys will remain in redis for - default value of zero makes this indefinite
  --redis_password string      If we need a password to login to the redis database, specify it
  --redis_url string           If we want to send dumps to a redis database we must set a valid URL
  --redis_write_timeout int    Timeout (seconds) for writing to redis (default 10)
  -t, --timeout int            Timeout for context - if none is specified a default background context will be used (default -1)
  --user_data_dir string         User data dir for browser data if we specify non headless mode (default "/tmp/chrome_dev_1")

Use "go-scraper [command] --help" for more information about a command.
```

## Fetch
```
Fetches all content from the URL in HTML format and writes it to stdout

Usage:
  go-scraper fetch [flags]

Flags:
  -h, --help                   help for fetch
      --href_selector string   Gets the first href for the node that match the specific selector
      --id_selector string     Gets the text that matches the specific selector by id
      --text_selector string   Gets and prints text for the desired selector and if not specified dump all content retrieved - can specify either an xpath or a css selector
  -u, --url string             URL that you are fetching HTML content for
      --wait_selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements - can specify either an xpath or a css selector

Global Flags:
  -a, --agent string           User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --captcha_click_selector string            The selector element to click for the captcha box (default "div.g-recaptcha")
      --captcha_click_sleep int               Time (seconds) we sleep after a captcha click, to allow the captcha challenge to get loaded into the iframe (default 5)
      --captcha_iframe_wait_selector string   The selector element to wait for the captcha iframe (default "body > div:nth-child(6) > div:nth-child(4) > iframe")
      --captcha_wait_selector string             The selector element to wait for so we can load the captcha box (default "div.re-captcha")
      --config string          config file (default is $HOME/.go-scraper.yaml)
      --detect_access_denied   If access denied is encoutered, then we will take a counter action
      --error_dump             Dumps current page contents on error
      --error_location         Logs the current URL that we have arrived at on error
      --headless               Use headless shell
      --redis_dump              Set this option for all dumps to go to the redis database that we connet to this app
      --redis_key_expiration int   The duration, in secondds that keys will remain in redis for - default value of zero makes this indefinite
      --redis_password string   If we need a password to login to the redis database, specify it
      --redis_url string        If we want to send dumps to a redis database we must set a valid URL
      --redis_write_timeout int Timeout (seconds) for writing to redis (default 10)
  -t, --timeout int            Timeout for context - if none is specified a default background context will be used (default -1)
  --user_data_dir string         User data dir for browser data if we specify non headless mode (default "/tmp/chrome_dev_1")
```

## Watch
```
This command provides sub-commands that we can run to take a particular action if the selectors (in the order of URLs specified) are found on the particular web-page (for the timeout set) and it will keep watching for the selectors at the set interval

Usage:
  go-scraper watch [command]

Available Commands:
  email       Emails if the desired criteria is met in watch

Flags:
      --captcha_click_selectors strings   Override the default captcha click selector for each URL or leave empty for that URL to just use (user provided) default from root level cmd
      --captcha_iframe_wait_selectors strings   Override captcha iframe wait selector for each URL
      --captcha_wait_selectors strings    Override the default captcha wait selector for each URL or leave empty for that URL to just use (user provided) default from root level cmd
      --check_selectors strings   Selectors that are used to check for the given expected-texts
      --check_types strings       The types of selectors for each check selector in order, which correspond to the ones in check_selectors - specify none to not use one for URL at that index
      --expected_texts strings    Pieces of texts that represent the normal state of an item - when the status is updated, the the desired user action will be taken
  -h, --help                     help for watch
  -i, --interval int             Interval (in seconds) to wait in between watching a selector (default 30)
      --notify_paths strings     A url path/domain sequence that indicates a more unique circumstance that we might want to be notified about
      --wait_selectors strings   All selectors, in order of URLs passed in, to wait for
      --urls strings             All URLs to watch

Global Flags:
  -a, --agent string             User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --captcha_click_selector string            The selector element to click for the captcha box (default "div.g-recaptcha")
      --captcha_click_sleep int               Time (seconds) we sleep after a captcha click, to allow the captcha challenge to get loaded into the iframe (default 5)
      --captcha_iframe_wait_selector string   The selector element to wait for the captcha iframe (default "body > div:nth-child(6) > div:nth-child(4) > iframe")
      --captcha_wait_selector string             The selector element to wait for so we can load the captcha box (default "div.re-captcha")
      --config string            config file (default is $HOME/.go-scraper.yaml)
      --detect_access_denied     If access denied is encoutered, then we will take a counter action
      --detect_captcha_box                    If a captcha box is encoutered, then we will take a counter action
      --detect_notify_path                    If a desired notify path is encountered, for a given URL, perform notification action
      --error_dump               Dumps current page contents on error
      --error_location           Logs the current URL that we have arrived at on error
      --headless                 Use headless shell
      --redis_dump               Set this option for all dumps to go to the redis database that we connet to this app
      --redis_key_expiration int The duration, in secondds that keys will remain in redis for - default value of zero makes this indefinite
      --redis_password string    If we need a password to login to the redis database, specify it
      --redis_url string         If we want to send dumps to a redis database we must set a valid URL
      --redis_write_timeout int  Timeout (seconds) for writing to redis (default 10)
  -t, --timeout int              Timeout for context - if none is specified a default background context will be used (default -1)
      --user_data_dir string         User data dir for browser data if we specify non headless mode (default "/tmp/chrome_dev_1")
```

## Email
```
This is one of the actions that can be taken for watch - it will send an email from the provided sender email to the receipient email

Usage:
  go-scraper watch email [flags]

Flags:
      --from string                  Email address to send message from
  -h, --help                         help for email
      --sender_password_env string   Password for the from email specified (specify as an environment variable)
      --subject string               Subject to be specified (default "Go-Scraper Watcher")
      --to string                    Email address to send message to

Global Flags:
  -a, --agent string                 User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --captcha_click_selector string     The selector element to click for the captcha box (default "div.g-recaptcha")
      --captcha_click_selectors strings   Override the default captcha click selector for each URL or leave empty for that URL to just use (user provided) default from root level cmd
      --captcha_click_sleep int               Time (seconds) we sleep after a captcha click, to allow the captcha challenge to get loaded into the iframe (default 5)
      --captcha_iframe_wait_selector string   The selector element to wait for the captcha iframe (default "body > div:nth-child(6) > div:nth-child(4) > iframe")
      --captcha_iframe_wait_selectors strings   Override captcha iframe wait selector for each URL
      --captcha_wait_selector string      The selector element to wait for so we can load the captcha box (default "div.re-captcha")
      --captcha_wait_selectors strings    Override the default captcha wait selector for each URL or leave empty for that URL to just use (user provided) default from root level cmd
      --check_selectors strings      Selectors that are used to check for the given expected_texts
      --check_types strings          The types of selectors for each check selector in order, which correspond to the ones in check_selectors - specify none to not use one for URL at that index
      --config string                config file (default is $HOME/.go-scraper.yaml)
      --detect_access_denied         If access denied is encoutered, then we will take a counter action
      --detect_captcha_box                    If a captcha box is encoutered, then we will take a counter action
      --detect_notify_path                    If a desired notify path is encountered, for a given URL, perform notification action
      --expected_texts strings       Pieces of texts that represent the normal state of an item - when the status is updated, the the desired user action will be taken
      --error_dump                   Dumps current page contents on error
      --error_location               Logs the current URL that we have arrived at on error
      --headless                     Use headless shell
      --redis_dump                   Set this option for all dumps to go to the redis database that we connet to this app
      --redis_key_expiration int     The duration, in secondds that keys will remain in redis for - default value of zero makes this indefinite
      --redis_password string        If we need a password to login to the redis database, specify it
      --redis_url string             If we want to send dumps to a redis database we must set a valid URL
      --redis_write_timeout int      Timeout (seconds) for writing to redis (default 10)
  -i, --interval int                 Interval (in seconds) to wait in between watching a selector (default 30)
      --notify_paths strings              A url path/domain sequence that indicates a more unique circumstance that we might want to be notified about
  -t, --timeout int                  Timeout for context - if none is specified a default background context will be used (default -1)
      --urls strings                 All URLs to watch
      --user_data_dir string         User data dir for browser data if we specify non headless mode (default "/tmp/chrome_dev_1")
```

## Discord

Inherits the same options as `Email` above from `Watch`.

```
This subcommand sends notifications to a Discord channel via a webhook

Usage:
  go-scraper watch discord [flags]

Flags:
      --discord_username string   Username to display in Discord notifications
  -h, --help                      help for discord
      --webhook string            Discord webhook URL to send notifications to
```

# Examples
Run from within headless-shell docker image to specify --headless  
Otherwise, it will need to run by calling into the chrome binary, which must
be installed on the machine you run this from

## Command-line args fetch all content example:
```
go-scraper --headless -t '10' fetch -u 'https://www.dsw.ca/en/ca/browse/sneakers/' --wait_selector
'.result-list' | cascadia -i -o -c 'div.result-list__tiles' -p Name='div.product-tile__detail-text' -p Price='div.product-pric
e' -d ','
2020/11/26 06:02:00 Fetching content from: https://www.dsw.ca/en/ca/browse/sneakers/
2020/11/26 06:02:00 Timeout specified: 10s
2020/11/26 06:02:00 Waiting on selector: .result-list
Name,Price,
Summits Sneaker,   $69.99    ,
Ward Lo Sneaker,  $74.99 $59.98    ,
Kaptir Sneaker,   $109.99    ,
Online Only Dighton - Bricelyn Sneaker,   $74.99    ,
Anzarun Sneaker,   $79.99    ,
Women's Ward Sneaker,   $79.96    ,
```

## Command-line args fetch print text example:
```
go-scraper --headless -t '10' fetch -u 'https://walmart.com/ip/Spider-Man-Miles-Morales-Launch-Edit
ion-Sony-PlayStation-5/238397352' --wait_selector 'div.prod-product-cta-add-to-cart.display-inline-block' --text_selector 'spa
n.spin-button-children'
2020/11/26 06:03:03 Fetching content from: https://walmart.com/ip/Spider-Man-Miles-Morales-Launch-Edition-Sony-PlayStation-5/238397352
2020/11/26 06:03:03 Timeout specified: 10s
2020/11/26 06:03:03 Waiting on selector: div.prod-product-cta-add-to-cart.display-inline-block
2020/11/26 06:03:03 Will print text for span.spin-button-children
Add to cart
```

## Command-line args watch and email example:
```
go-scraper --headless -t 10 watch --urls 'https://walmart.com/ip/Spider-Man-Miles-Morales-Launch-Edition-Sony-PlayStation-5/238397352' --wait_selectors 'div.prod-product-cta-add-to-cart.display-inline-block' -i 30 email --from 'vrajendrantester@gmail.com' --to 'vishnu.raj.1993@gmail.com'
2020/11/26 22:43:44 Sending with subject Go-Scraper Watcher
2020/11/26 22:43:44 Sending from email vrajendrantester@gmail.com
2020/11/26 22:43:44 Sending to email vishnu.raj.1993@gmail.com
2020/11/26 22:43:44 Will check for updates every 30 seconds
2020/11/26 22:43:44 Timeout specified: 10s
2020/11/26 22:43:50 Emailed vishnu.raj.1993@gmail.com successfully
2020/11/26 22:44:20 Timeout specified: 10s
2020/11/26 22:44:27 Emailed vishnu.raj.1993@gmail.com successfully
```

# Other

## Useful command-line tools to run with:
https://github.com/suntong/cascadia

## Docker image to run with headless:
https://hub.docker.com/r/chromedp/headless-shell/

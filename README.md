# Purpose
Command-line tool to interact with HTML content retrieved from dynamic web pages  
Uses the chromedp library:  
https://github.com/chromedp/chromedp

# Usage

## General:
```
Allows you to request data from dynamic web pages and interact with it

Usage:
  go-dynamic-fetch [command]

Available Commands:
  fetch       Write the HTML content for the URL to stdout
  help        Help about any command

Flags:
  -a, --agent string    User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
  -h, --help            help for go-dynamic-fetch
  -t, --timeout int     Timeout for context - if none is specified a default background context will be used (default -1)

Use "go-dynamic-fetch [command] --help" for more information about a command.
```

## Fetch:
```
Fetches all content from the URL in HTML format and writes it to stdout

Usage:
  go-dynamic-fetch fetch [flags]

Flags:
  -h, --help                   help for fetch
      --text-selector string   Gets and prints text for the desired selector and if not specified dump all content retrieved
  -u, --url string             URL that you are fetching HTML content for
      --wait-selector string   Selector for element to wait for - if not specified we do not wait and just dump static elements

Global Flags:
  -a, --agent string    User agent to request as - if not specified the default is used (default "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_14_5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/77.0.3830.0 Safari/537.36")
      --config string   config file (default is $HOME/.go-dynamic-fetch.yaml)
      --headless        Use headless shell
  -t, --timeout int     Timeout for context - if none is specified a default background context will be used (default -1)
```

# Examples
Run from within headless-shell docker image to specify --headless  
Otherwise, it will need to run by calling into the chrome binary, which must
be installed on the machine you run this from

## Command-line args fetch all content example:
```
go-dynamic-fetch --headless -t '10' fetch -u 'https://www.dsw.ca/en/ca/browse/sneakers/' --wait-selector
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
go-dynamic-fetch --headless -t '10' fetch -u 'https://walmart.com/ip/Spider-Man-Miles-Morales-Launch-Edit
ion-Sony-PlayStation-5/238397352' --wait-selector 'div.prod-product-cta-add-to-cart.display-inline-block' --text-selector 'spa
n.spin-button-children'
2020/11/26 06:03:03 Fetching content from: https://walmart.com/ip/Spider-Man-Miles-Morales-Launch-Edition-Sony-PlayStation-5/238397352
2020/11/26 06:03:03 Timeout specified: 10s
2020/11/26 06:03:03 Waiting on selector: div.prod-product-cta-add-to-cart.display-inline-block
2020/11/26 06:03:03 Will print text for span.spin-button-children
Add to cart
```

# Other

## Useful command-line tools to run with:
https://github.com/suntong/cascadia

## Docker image to run with headless:
https://hub.docker.com/r/chromedp/headless-shell/
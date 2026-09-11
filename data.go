package main

type Resource struct {
	ID          string
	Name        string
	URL         string
	Description string
}

type Language struct {
	Slug      string
	Name      string
	Resources []Resource
}

var languages = map[string]Language{
	"python": {
		Slug: "python",
		Name: "Python",
		Resources: []Resource{
			{ID: "python-docs", Name: "Official Python Docs", URL: "https://docs.python.org/3/", Description: "Official documentation"},
			{ID: "python-getting-started", Name: "Python.org Beginner's Guide", URL: "https://www.python.org/about/gettingstarted/", Description: "Great starting point for newcomers"},
			{ID: "python-realpython", Name: "Real Python", URL: "https://realpython.com", Description: "In-depth tutorials and articles"},
			{ID: "python-freecodecamp", Name: "freeCodeCamp Python Course", URL: "https://www.freecodecamp.org/learn/scientific-computing-with-python/", Description: "Free hands-on course"},
			{ID: "python-automate-boring-stuff", Name: "Automate the Boring Stuff", URL: "https://automatetheboringstuff.com", Description: "Free beginner-friendly book"},
			{ID: "python-w3schools", Name: "W3Schools Python", URL: "https://www.w3schools.com/python/", Description: "Quick reference and simple exercises"},
		},
	},
	"go": {
		Slug: "go",
		Name: "GO",
		Resources: []Resource{
			{ID: "go-tour", Name: "A Tour of Go", URL: "https://go.dev/tour/", Description: "Official interactive intro"},
			{ID: "go-by-example", Name: "Go by Example", URL: "https://gobyexample.com", Description: "Hands-on annotated examples"},
			{ID: "go-docs", Name: "Go Documentation", URL: "https://go.dev/doc/", Description: "Official docs"},
			{ID: "go-effective-go", Name: "Effective Go", URL: "https://go.dev/doc/effective_go", Description: "Best practices from the Go team"},
			{ID: "go-learn-with-tests", Name: "Learn Go with Tests", URL: "https://quii.gitbook.io/learn-go-with-tests/", Description: "Learn Go via test-driven development"},
			{ID: "go-gophercises", Name: "Gophercises", URL: "https://gophercises.com", Description: "Free coding exercises for Go"},
		},
	},
	"css": {
		Slug: "css",
		Name: "CSS",
		Resources: []Resource{
			{ID: "css-mdn", Name: "MDN CSS Docs", URL: "https://developer.mozilla.org/en-US/docs/Web/CSS", Description: "Official reference and guides"},
			{ID: "css-tricks", Name: "CSS-Tricks", URL: "https://css-tricks.com", Description: "Tips, tricks, and deep dives"},
			{ID: "css-w3schools", Name: "W3Schools CSS", URL: "https://www.w3schools.com/css/", Description: "Beginner-friendly reference"},
			{ID: "css-flexbox-froggy", Name: "Flexbox Froggy", URL: "https://flexboxfroggy.com", Description: "Learn Flexbox through a game"},
			{ID: "css-grid-garden", Name: "Grid Garden", URL: "https://cssgridgarden.com", Description: "Learn CSS Grid through a game"},
			{ID: "css-freecodecamp", Name: "freeCodeCamp CSS Course", URL: "https://www.freecodecamp.org/learn/2022/responsive-web-design/", Description: "Free structured course"},
		},
	},
	"http": {
		Slug: "http",
		Name: "HTTP",
		Resources: []Resource{
			{ID: "http-mdn-overview", Name: "MDN HTTP Overview", URL: "https://developer.mozilla.org/en-US/docs/Web/HTTP", Description: "Comprehensive protocol overview"},
			{ID: "http-mdn-messages", Name: "MDN HTTP Messages", URL: "https://developer.mozilla.org/en-US/docs/Web/HTTP/Messages", Description: "How requests/responses work"},
			{ID: "http-status-dogs", Name: "HTTP Status Dogs", URL: "https://httpstatusdogs.com", Description: "Fun way to learn status codes"},
			{ID: "http-web-dev-learn", Name: "web.dev Learn HTTP", URL: "https://web.dev/learn", Description: "Google's web fundamentals hub"},
			{ID: "http-freecodecamp-guide", Name: "freeCodeCamp HTTP Guide", URL: "https://www.freecodecamp.org/news/http-request-methods-explained/", Description: "Plain-language explanation of HTTP methods"},
			{ID: "http-rfc9110", Name: "RFC 9110 (HTTP Semantics)", URL: "https://httpwg.org/specs/rfc9110.html", Description: "The official spec, for the curious"},
		},
	},
	"https": {
		Slug: "https",
		Name: "HTTPS",
		Resources: []Resource{
			{ID: "https-mdn", Name: "MDN: What is HTTPS?", URL: "https://developer.mozilla.org/en-US/docs/Glossary/HTTPS", Description: "Beginner-friendly explanation"},
			{ID: "https-cloudflare", Name: "Cloudflare: What is HTTPS?", URL: "https://www.cloudflare.com/learning/ssl/what-is-https/", Description: "Clear visual explanation"},
			{ID: "https-comic", Name: "How HTTPS Works (comic)", URL: "https://howhttps.works", Description: "Fun comic-style explainer"},
			{ID: "https-letsencrypt", Name: "Let's Encrypt Docs", URL: "https://letsencrypt.org/docs/", Description: "Learn by getting a real cert"},
			{ID: "https-digitalocean", Name: "DigitalOcean: SSL/TLS Explained", URL: "https://www.digitalocean.com/community/tutorials/openssl-essentials-working-with-ssl-certificates-private-keys-and-csrs", Description: "Practical tutorial on certificates"},
			{ID: "https-web-dev-why", Name: "web.dev: Why HTTPS Matters", URL: "https://web.dev/why-https-matters/", Description: "Google's take on HTTPS importance"},
		},
	},
	"java": {
		Slug: "java",
		Name: "JAVA",
		Resources: []Resource{
			{ID: "java-oracle-tutorials", Name: "Oracle Java Tutorials", URL: "https://docs.oracle.com/javase/tutorial/", Description: "Official tutorials"},
			{ID: "java-w3schools", Name: "W3Schools Java", URL: "https://www.w3schools.com/java/", Description: "Beginner-friendly reference"},
			{ID: "java-codecademy", Name: "Codecademy Java Course", URL: "https://www.codecademy.com/learn/learn-java", Description: "Interactive course"},
			{ID: "java-baeldung", Name: "Baeldung", URL: "https://www.baeldung.com", Description: "In-depth Java articles and guides"},
			{ID: "java-mooc-helsinki", Name: "Java Programming MOOC (Helsinki)", URL: "https://java-programming.mooc.fi", Description: "Free university-level course"},
			{ID: "java-freecodecamp", Name: "freeCodeCamp Java Course", URL: "https://www.freecodecamp.org/news/learn-java-full-course/", Description: "Free full video course"},
		},
	},
	"cpp": {
		Slug: "cpp",
		Name: "C++",
		Resources: []Resource{
			{ID: "cpp-cplusplus-tutorial", Name: "cplusplus.com Tutorial", URL: "https://cplusplus.com/doc/tutorial/", Description: "Classic beginner tutorial"},
			{ID: "cpp-cppreference", Name: "cppreference.com", URL: "https://en.cppreference.com", Description: "The definitive reference"},
			{ID: "cpp-learncpp", Name: "LearnCpp.com", URL: "https://www.learncpp.com", Description: "Excellent structured free course"},
			{ID: "cpp-w3schools", Name: "W3Schools C++", URL: "https://www.w3schools.com/cpp/", Description: "Beginner-friendly reference"},
			{ID: "cpp-geeksforgeeks", Name: "GeeksforGeeks C++", URL: "https://www.geeksforgeeks.org/c-plus-plus/", Description: "Tutorials plus practice problems"},
			{ID: "cpp-freecodecamp", Name: "freeCodeCamp C++ Course", URL: "https://www.freecodecamp.org/news/learn-c-plus-plus-programming-course/", Description: "Free full video course"},
		},
	},
	"php": {
		Slug: "php",
		Name: "PHP",
		Resources: []Resource{
			{ID: "php-manual", Name: "Official PHP Manual", URL: "https://www.php.net/manual/en/", Description: "Official documentation"},
			{ID: "php-w3schools", Name: "W3Schools PHP", URL: "https://www.w3schools.com/php/", Description: "Beginner-friendly reference"},
			{ID: "php-the-right-way", Name: "PHP The Right Way", URL: "https://phptherightway.com", Description: "Best practices guide"},
			{ID: "php-laracasts", Name: "Laracasts", URL: "https://laracasts.com", Description: "Great PHP and Laravel screencasts"},
			{ID: "php-codecademy", Name: "Codecademy PHP Course", URL: "https://www.codecademy.com/learn/learn-php", Description: "Interactive course"},
			{ID: "php-freecodecamp", Name: "freeCodeCamp PHP Course", URL: "https://www.freecodecamp.org/news/php-tutorial-for-beginners/", Description: "Free full video course"},
		},
	},
	"sql": {
		Slug: "sql",
		Name: "SQL",
		Resources: []Resource{
			{ID: "sql-w3schools", Name: "W3Schools SQL", URL: "https://www.w3schools.com/sql/", Description: "Beginner-friendly reference with try-it editor"},
			{ID: "sql-sqlzoo", Name: "SQLZoo", URL: "https://sqlzoo.net", Description: "Interactive SQL practice"},
			{ID: "sql-mode", Name: "Mode SQL Tutorial", URL: "https://mode.com/sql-tutorial/", Description: "Practical, real-world focused"},
			{ID: "sql-khan-academy", Name: "Khan Academy SQL", URL: "https://www.khanacademy.org/computing/computer-programming/sql", Description: "Free structured course"},
			{ID: "sql-postgresqltutorial", Name: "PostgreSQL Tutorial", URL: "https://www.postgresqltutorial.com", Description: "Great if you want a specific DB"},
			{ID: "sql-freecodecamp", Name: "freeCodeCamp SQL Course", URL: "https://www.freecodecamp.org/learn/relational-database/", Description: "Free hands-on course"},
		},
	},
	"javascript": {
		Slug: "javascript",
		Name: "JAVASCRIPT",
		Resources: []Resource{
			{ID: "js-mdn", Name: "MDN JavaScript Docs", URL: "https://developer.mozilla.org/en-US/docs/Web/JavaScript", Description: "Official reference and guides"},
			{ID: "js-javascript-info", Name: "JavaScript.info", URL: "https://javascript.info", Description: "Excellent modern JS tutorial"},
			{ID: "js-freecodecamp", Name: "freeCodeCamp JavaScript Course", URL: "https://www.freecodecamp.org/learn/javascript-algorithms-and-data-structures/", Description: "Free hands-on course"},
			{ID: "js-eloquent", Name: "Eloquent JavaScript", URL: "https://eloquentjavascript.net", Description: "Free in-depth book"},
			{ID: "js-w3schools", Name: "W3Schools JavaScript", URL: "https://www.w3schools.com/js/", Description: "Beginner-friendly reference"},
			{ID: "js-odin-project", Name: "The Odin Project", URL: "https://www.theodinproject.com", Description: "Full free curriculum, JS-focused"},
		},
	},
}

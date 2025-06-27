
## Directory Structure

Rules from https://github.com/golang-standards/project-layout

### `/cmd`
Main applications for this project.

The directory name for each application should match the name of the executable you want to have (e.g., /cmd/myapp).

Don't put a lot of code in the application directory. If you think the code can be imported and used in other projects, then it should live in the /pkg directory. You'll be surprised what others will do, so be explicit about your intentions!

It's common to have a small main function that imports and invokes the code from the /pkg directories and nothing else.

### `/pkg`
Our own development codes that's ok to use by external applications (e.g., `/pkg/forwardpropagation`). Other upper codes (e.g., applications in `/cmd`) will import these libraries expecting them to work, so think twice before you put something here :-)

### `/vendor`
Application dependencies (e.g. `lattigv6`). The `go mod vendor` command will create the `/vendor` directory for you.  
Don't commit your application dependencies if you are building a library (you don't have to do anything about this because this directory is exempted by `.gitignore` while committing)
**There are cases where we need to modify the underlying dependencies (and this might be frequent), this should be committed to the `/modifications`, with sub-directories exactly the same as that of the original dependency.** 

### `/configs`
Configuration file templates or default configs.

### `/test`
Additional external test apps and test data. Feel free to structure the `/test` directory anyway you want. For bigger projects it makes sense to have a data subdirectory. For example, you can have `/test/data` or `/test/testdata` if you need Go to ignore what's in that directory. Note that Go will also ignore directories or files that begin with "." or "_", so you have more flexibility in terms of how you name your test data directory.

### `/docs`
Design and user documents (in addition to your godoc generated documentation).

### Others
Note that datasets should not be stored in the github project, it is encouraged to commit codes for drawing datasets on-the-fly.  

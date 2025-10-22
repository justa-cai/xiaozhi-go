set -e
set -x
rm -f log.txt

# Function to log to both file and console
log_to_both() {
    echo "$1" | tee -a log.txt
}

# Build and run with dual logging
log_to_both "Building xiaozhi..."
if make build; then
    log_to_both "Build successful. Starting xiaozhi with wake word detection..."

    # Run xiaozhi with tee to output to both console and log file
    ./bin/xiaozhi -wakeword -log-level info 2>&1 | tee -a log.txt
else
    log_to_both "Build failed!"
    exit 1
fi


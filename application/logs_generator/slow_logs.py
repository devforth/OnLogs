if __name__ == "__main__":
    import time
    import random
    import logging

    logging.basicConfig(level=logging.INFO)

    logging.info("This is a log message that will be followed by a delay.")
    time.sleep(random.randint(1, 5))
    logging.info("This is a log message after the initial delay.")

    while True:
        seconds = random.randint(1, 5)
        time.sleep(seconds)
        logging.info("This is a slow log message after waiting for %d seconds.", seconds)

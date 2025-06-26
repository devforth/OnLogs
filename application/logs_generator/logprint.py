import random
import string
import time


def main():
    output = 0
    while True:
        str_len = random.randint(50, 100)
        output = ""
        for _ in range(str_len):
            output += random.choice(string.ascii_letters + ' ' + string.digits + ' ')
        output = random.choice(["INFO ", "DEBUG ", "WARNING ", "ERROR ", "", ""]) + output
        output = random.choice([output, output, output, " "])
        print(output)
        time.sleep(0.01)


main()

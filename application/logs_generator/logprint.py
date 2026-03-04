import random
import string
import time

ANSI_RESET = "\x1b[0m"
ANSI_DIM = "\x1b[2m"
ANSI_BOLD = "\x1b[1m"
ANSI_WHITE = "\x1b[37m"
ANSI_CYAN = "\x1b[36m"
ANSI_YELLOW = "\x1b[33m"
ANSI_RED = "\x1b[31m"
ANSI_GREEN = "\x1b[32m"
ANSI_MAGENTA = "\x1b[35m"

PREFIXES = [
    f"{ANSI_WHITE}INFO{ANSI_DIM} ",
    f"{ANSI_CYAN}DEBUG{ANSI_RESET} ",
    f"{ANSI_YELLOW}WARNING{ANSI_BOLD} ",
    f"{ANSI_RED}ERROR{ANSI_RESET} ",
]

LINE_STYLES = [
    ANSI_RESET,
    ANSI_DIM,
    ANSI_BOLD,
    ANSI_CYAN,
    ANSI_YELLOW,
    ANSI_RED,
    ANSI_GREEN,
    ANSI_MAGENTA,
    ANSI_WHITE,
]


def random_word(min_len=3, max_len=14):
    length = random.randint(min_len, max_len)
    alphabet = string.ascii_letters + string.digits
    return "".join(random.choice(alphabet) for _ in range(length))


def stylize_token(token):
    # Colorize regular line content too, not only level prefixes.
    if random.random() < 0.35:
        return random.choice(LINE_STYLES) + token + ANSI_RESET
    return token


def build_line_tokens(target_tokens):
    tokens = []
    for _ in range(target_tokens):
        token = stylize_token(random_word())
        if random.random() < 0.18:
            token = "\t" + token
        tokens.append(token)
    return tokens


def build_single_line():
    token_count = random.randint(8, 35)
    tokens = build_line_tokens(token_count)
    line_prefix = random.choice(PREFIXES) if random.random() < 0.9 else ""
    return line_prefix + " ".join(tokens)


def build_log_message():
    # Sometimes produce very long logs.
    if random.random() < 0.2:
        line_count = random.randint(4, 10)
    else:
        line_count = random.randint(1, 3)

    lines = [build_single_line() for _ in range(line_count)]
    message = "\n".join(lines)

    # Occasionally create a long single-line tail.
    if random.random() < 0.25:
        long_tail = " ".join(build_line_tokens(random.randint(60, 140)))
        message = f"{message}\n{random.choice(PREFIXES)}{long_tail}"

    return message


def build_newline_ansi_edge_case():
    # Explicit parser-stress case:
    # 1) apply color
    # 2) include newline
    # 3) apply DIM/BOLD
    # 4) close with RESET
    color = random.choice([ANSI_RED, ANSI_YELLOW, ANSI_CYAN, ANSI_GREEN, ANSI_MAGENTA, ANSI_WHITE])
    level = random.choice(PREFIXES)
    first_line = f"{level}{color}EDGE-CASE-START {random_word(6, 14)}"
    second_line = (
        f"{ANSI_DIM}{ANSI_BOLD}EDGE-CASE-AFTER-NEWLINE\t{random_word(8, 20)} "
        f"{random_word(8, 20)}{ANSI_RESET}"
    )
    return f"{first_line}\n{second_line}"


def main():
    while True:
        if random.random() < 0.12:
            output = build_newline_ansi_edge_case()
        else:
            output = build_log_message()
        if random.random() < 0.06:
            output = " "
        print(output, flush=True)
        time.sleep(0.1)


main()

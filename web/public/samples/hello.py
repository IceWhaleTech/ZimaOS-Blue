# A simple hello world program with a bug
# Can you find and fix the error?

def greet(name):
    """Greet a person by name"""
    message = "Hello, " + name + "!"
    return mesage  # Typo: should be 'message'

def main():
    names = ["Alice", "Bob", "Charlie"]
    for name in names:
        print(greet(name))

if __name__ == "__main__"
    main()  # Missing colon after __main__

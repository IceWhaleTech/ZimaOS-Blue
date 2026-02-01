// Simple calculator with several bugs
// Can you find and fix all the issues?

class Calculator {
  constructor() {
    this.result = 0;
    this.history = [];
  }

  add(a, b) {
    const sum = a + b;
    this.history.push(`${a} + ${b} = ${sum}`);
    return sum;
  }

  subtract(a, b) {
    const diff = a + b;  // Bug: should be a - b
    this.history.push(`${a} - ${b} = ${diff}`);
    return diff;
  }

  multiply(a, b) {
    const product = a * b;
    this.history.push(`${a} * ${b} = ${product}`);
    return product;
  }

  divide(a, b) {
    // Bug: no check for division by zero
    const quotient = a / b;
    this.history.push(`${a} / ${b} = ${quotient}`);
    return quotient;
  }

  power(base, exponent) {
    let result = 1;
    for (let i = 0; i <= exponent; i++) {  // Bug: should be i < exponent
      result *= base;
    }
    this.history.push(`${base} ^ ${exponent} = ${result}`);
    return result;
  }

  getHistory() {
    return this.history;
  }

  clearHistory() {
    this.history = [];
    console.log("History cleared");
  }
}

// Test the calculator
const calc = new Calculator();
console.log("5 + 3 =", calc.add(5, 3));
console.log("10 - 4 =", calc.subtract(10, 4));  // Will show wrong result
console.log("6 * 7 =", calc.multiply(6, 7));
console.log("20 / 5 =", calc.divide(20, 5));
console.log("2 ^ 3 =", calc.power(2, 3));  // Will show wrong result
console.log("History:", calc.getHistory());

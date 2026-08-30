function hasDeadCode() {
    return 42;
    console.log("This is unreachable"); // Dead code after return
}

function hasBreak() {
    for (let i = 0; i < 10; i++) {
        if (i === 5) {
            break;
            console.log("After break"); // Dead code after break
        }
    }
}

function complexFunction(x, y) {
    if (x > 0) {
        if (y > 0) {
            return x + y;
        } else {
            return x - y;
        }
    } else {
        return 0;
    }

    console.log("Unreachable"); // Dead code after return in all branches
}

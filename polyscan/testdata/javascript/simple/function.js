function hello() {
    return 42;
}

function greet(name) {
    if (name) {
        return "Hello, " + name;
    } else {
        return "Hello, stranger";
    }
}

const add = (a, b) => {
    return a + b;
};

async function fetchData() {
    const data = await fetch('/api/data');
    return data;
}

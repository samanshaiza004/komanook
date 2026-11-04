export function nextIndex(index, length) {
    return length === 0 ? -1 : (index + 1 + length) % length;
}
export function previousIndex(index, length) {
    return length === 0 ? -1 : (index - 1 + length) % length;
}
export function edgeIndex(edge, length) {
    if (length === 0)
        return -1;
    return edge === "first" ? 0 : length - 1;
}

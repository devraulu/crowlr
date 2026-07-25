import "./App.css";

function App() {
  return (
    <>
      <h1>
        <a href="/">crowlr</a>
      </h1>
      <form className="search-box">
        <input
          type="search"
          name="q"
          placeholder="ask questions about the crawled data..."
        />
        <button id="submit-btn" type="submit">
          GO
        </button>
      </form>
      <div id="results"></div>
    </>
  );
}

export default App;

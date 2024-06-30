import React from 'react';
import {Link, Route, Routes} from 'react-router-dom';
import Home from './pages/Home';
import About from './pages/About';
import './App.css'

function App() {
    return (
        <>
            <h1 className="text-3xl font-bold underline">Top</h1>
            <nav>
                <ul>
                    <li><Link to="/">Home</Link></li>
                    <li><Link to="/about">About</Link></li>
                </ul>
            </nav>
            <Routes>
                <Route path="/" element={<Home/>}/>
                <Route path="/about" element={<About/>}/>
            </Routes>
        </>
    )
}

export default App

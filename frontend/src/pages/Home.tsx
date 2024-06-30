import React, {useState} from 'react';

// Import Swiper styles
import "swiper/css";
import "swiper/css/effect-coverflow";
import 'swiper/css/navigation';
import 'swiper/css/pagination';
import SwipeableCard from "../components/SwipeableCard.tsx";

// Initialize Swiper modules
// SwiperCore.use([Navigation, Pagination]);

function Home() {
    const [cards, setCards] = useState([
        {id: 1, content: 'Card 1'},
        {id: 2, content: 'Card 2'},
        {id: 3, content: 'Card 3'},
    ]);

    const [currentCardIndex, setCurrentCardIndex] = useState(0);

    const handleSwiped = (dir: string) => {
        if (dir === 'left') {
            setCurrentCardIndex((prev) => (prev > 0 ? prev - 1 : cards.length - 1));
        } else if (dir === 'right') {
            setCurrentCardIndex((prev) => (prev + 1) % cards.length);
        }
    };

    return (
        <div className="swipeable-cards-container">
            {cards.length > 0 && (
                <SwipeableCard
                    key={cards[currentCardIndex].id}
                    content={cards[currentCardIndex].content}
                    onSwiped={handleSwiped}
                />
            )}
        </div>
    );
}

export default Home;